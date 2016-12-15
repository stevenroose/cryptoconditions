package cryptoconditions

import (
	"crypto/sha256"
	"fmt"
)

//TODO do we really need to incorporate for unfulfilled ones? it makes this file so much more complicated

// FfThresholdSha256 implements the Threshold-SHA-256 fulfillment.
type FfThresholdSha256 struct {
	Threshold uint16

	SubFulfillments []Fulfillment `asn1:"choice:fulfillment"`
	SubConditions   []Condition   `asn1:"choice:condition"`
}

//TODO NORMALIZE METHOD that makes sure the FF is of minimal size by replacing (threshold - nbFulfillments) fulfillments
// with their conditions, choosing those fulfillments that have the biggest (fulfillmentSize - conditionSize).

// NewFfThresholdSha256 creates a new FfThresholdSha256 fulfillment.
func NewFfThresholdSha256(threshold uint16, subFulfillments []Fulfillment, subConditions []Condition) *FfThresholdSha256 {
	return &FfThresholdSha256{
		Threshold:       threshold,
		SubFulfillments: subFulfillments,
		SubConditions:   subConditions,
	}
}

func (ff *FfThresholdSha256) ConditionType() ConditionType {
	return CTThresholdSha256
}

func (ff *FfThresholdSha256) Condition() Condition {
	return NewCompoundCondition(ff.ConditionType(), ff.fingerprint(), ff.maxFulfillmentLength(), ff.subConditionTypeSet())
}

func (ff *FfThresholdSha256) fingerprint() []byte {
	subConditions := make([]Condition, len(ff.SubFulfillments))
	for i, sff := range ff.SubFulfillments {
		subConditions[i] = sff.Condition()
	}
	content := struct {
		threshold     uint16
		subConditions []Condition `asn1:"set,choice:condition"`
	}{
		threshold:     ff.Threshold,
		subConditions: subConditions,
	}

	encoded, err := Asn1Context.Encode(content)
	if err != nil {
		//TODO
		panic(err)
	}
	hash := sha256.Sum256(encoded)
	return hash[:]
}

func (ff *FfThresholdSha256) maxFulfillmentLength() int {
	//TODO IMPLEMENT
	return 0
}

func (ff *FfThresholdSha256) subConditionTypeSet() *ConditionTypeSet {
	set := new(ConditionTypeSet)
	for _, sff := range ff.SubFulfillments {
		set.addRelevant(sff)
	}
	for _, sc := range ff.SubConditions {
		set.addRelevant(sc)
	}
	return set
}

func (ff *FfThresholdSha256) Validate(condition Condition, message []byte) error {
	if !matches(ff, condition) {
		return fulfillmentDoesNotMatchConditionError
	}

	th := int(ff.Threshold)
	if th == 0 {
		return nil
	}

	// Check if we have enough fulfillments.
	if len(ff.SubFulfillments) < th {
		return fmt.Errorf("Not enough fulfillments: %v of %v", len(ff.SubFulfillments), th)
	}

	// Try to verify the fulfillments one by one.
	for _, ff := range ff.SubFulfillments {
		if ff.Validate(nil, message) == nil {
			th--
			if th == 0 {
				break
			}
		}
	}

	if th != 0 {
		return fmt.Errorf("Could only verify %v of %v fulfillments", int(ff.Threshold)-th, th)
	}
	return nil
}

//
//// weightedFulfillmentInfo is a weightedFulfillment with some of its info cached for use by
//// calculateSmallestValidFulfillmentSet
//type weightedSubFulfillmentInfo struct {
//	*weightedSubFulfillment
//	// the index in the original ff set of the ff this info corresponds to
//	index int
//	// the size of the fulfillment to be included and the size of the condition in the case it's not
//	size, omitSize uint32
//}
//
//// weightedFulfillments is a slice of weightedFulfillments that implements sort.Interface to
//// sort them by weight in descending order.
//type weightedSubFulfillmentInfoSorter []*weightedSubFulfillmentInfo
//
//func (w weightedSubFulfillmentInfoSorter) Len() int           { return len(w) }
//func (w weightedSubFulfillmentInfoSorter) Less(i, j int) bool { return w[j].weight < w[i].weight }
//func (w weightedSubFulfillmentInfoSorter) Swap(i, j int)      { w[i], w[j] = w[j], w[i] }
//
//// State object used for the calculation of the smallest valid set of fulfillments.
//type smallestValidFulfillmentSetCalculatorState struct {
//	index int
//	size  uint32
//	set   []int
//}
//
//// hasIndex checks if a certain fulfillment index is in smallestValidFulfillmentSetCalculator.set
//func (c smallestValidFulfillmentSetCalculatorState) hasIndex(idx int) bool {
//	for _, v := range c.set {
//		if v == idx {
//			return true
//		}
//	}
//	return false
//}
//
//// calculateSmallestValidFulfillmentSet calculates the smallest valid set of sub-fulfillments that reach the given
//// threshold. The method works recursively and keeps the state of the current recursion in the a
//// smallestValidFulfillmentSetCalculatorState object.
//func calculateSmallestValidFulfillmentSet(threshold uint32, sffs []*weightedSubFulfillmentInfo,
//	state *smallestValidFulfillmentSetCalculatorState) (*smallestValidFulfillmentSetCalculatorState, error) {
//	// Threshold reached, so the set we have is enough.
//	if threshold <= 0 {
//		return &smallestValidFulfillmentSetCalculatorState{
//			size: state.size,
//			set:  state.set,
//		}, nil
//	}
//
//	// We iterated through the list of sub-fulfillments and we did not find a valid set -> impossible.
//	if state.index >= len(sffs) {
//		return &smallestValidFulfillmentSetCalculatorState{
//			size: maxWeight,
//		}, nil
//	}
//
//	// Regular case: we calculate the set if we would include or not include the next sub-fulfillment
//	// and then pick the choice with the lowest size.
//	nextSff := sffs[state.index]
//
//	withoutNext, err := calculateSmallestValidFulfillmentSet(
//		threshold,
//		sffs,
//		&smallestValidFulfillmentSetCalculatorState{
//			index: state.index + 1,
//			size:  state.size + nextSff.omitSize,
//			set:   state.set,
//		},
//	)
//	if err != nil {
//		return nil, errors.Wrapf(err, "Failed to calculate smallest valid fulfillment set (without index %v)", state.index)
//	}
//
//	// If not fulfilled, we can only consider the case to not include it.
//	if !nextSff.isFulfilled {
//		return withoutNext, nil
//	}
//
//	withNext, err := calculateSmallestValidFulfillmentSet(
//		threshold-nextSff.weight,
//		sffs,
//		&smallestValidFulfillmentSetCalculatorState{
//			index: state.index + 1,
//			size:  state.size + nextSff.size,
//			set:   append(state.set, nextSff.index),
//		},
//	)
//	if err != nil {
//		return nil, errors.Wrapf(err, "Failed to calculate smallest valid fulfillment set (with index %v", state.index)
//	}
//
//	// return the smallest
//	if withNext.size < withoutNext.size {
//		return withNext, nil
//	} else {
//		return withoutNext, nil
//	}
//}
//
//
//func (ff *FfThresholdSha256) String() string {
//	uri, err := Uri(ff)
//	if err != nil {
//		return "!Could not generate Fulfillment's URI!"
//	}
//	return uri
//}
//
//func (ff *FfThresholdSha256) calculateMaxFulfillmentLength() (uint32, error) {
//	// build a list with the fulfillment infos needed for the calculation
//	sffs := make([]*weightedSubFulfillmentInfo, len(ff.SubFulfillments))
//	for i, sff := range ff.SubFulfillments {
//		// size: serialize fulfillment (can only do this when we have it)
//		var size uint32
//		if sff.isFulfilled {
//			counter := new(writeCounter)
//			if err := SerializeFulfillment(counter, sff.ff); err != nil {
//				return 0, errors.Wrap(err, "Failed to serialize sub-fulfillment")
//			}
//			size = uint32(counter.Counter())
//		}
//
//		// omit size: serialize condition
//		sffCond, err := sff.Condition()
//		if err != nil {
//			return 0, errors.Wrap(err, "Failed to generate condition of sub-fulfillment")
//		}
//		counter := new(writeCounter)
//		if err := SerializeCondition(counter, sffCond); err != nil {
//			return 0, errors.Wrap(err, "Failed to serialize condition of sub-filfillment")
//		}
//		omitSize := uint32(counter.Counter())
//
//		sffs[i] = &weightedSubFulfillmentInfo{
//			weightedSubFulfillment: sff,
//			index:    i,
//			size:     uint32(size),
//			omitSize: uint32(omitSize),
//		}
//	}
//
//	// cast to weightedFulfillmentInfoSorter so that we can sort by weight
//	sorter := weightedSubFulfillmentInfoSorter(sffs)
//	sort.Sort(sorter)
//
//	sffsWorstLength, err := calculateWorstCaseSffsSize(ff.Threshold, sffs, 0)
//
//	if err != nil {
//		return 0, errors.New("Insufficient subconditions/weights to meet the threshold.")
//	}
//
//	// calculate the size of the remainder of the fulfillment
//	counter := new(writeCounter)
//	// JS counts threshold as uint32 instead of VarUInt
//	counter.Skip(4)
//	writeVarUInt(counter, len(ff.SubFulfillments))
//	for _, sff := range ff.SubFulfillments {
//		// features bitmask is 1 byte
//		counter.Skip(1)
//		if sff.weight != 1 {
//			// weight is uint32, so 4 bytes
//			counter.Skip(4)
//		}
//	}
//	// add the worst case total length of the serialized fulfillments and conditions
//	counter.Skip(int(sffsWorstLength))
//
//	return uint32(counter.Counter()), nil
//}
//
//// calculateWorstCaseSffsSize used in the calculation below
//var calculateWorstCaseSffsSizeError = errors.New("Unable to canculate size")
//
//// calculateWorstCaseSffsLength returns the worst case total length of the sub-fulfillments.
//// The weighted sub-fulfillments must be ordered by weight descending.
//// It returns any error when it was impossible to find one.
//func calculateWorstCaseSffsSize(threshold uint32, sffs []*weightedSubFulfillmentInfo, index int) (uint32, error) {
//	if threshold <= 0 {
//		// threshold reached, no additional fulfillments need to be added
//		return 0, nil
//	} else if index < len(sffs) {
//		// calculate whether including or excluding the fulfillment increases the size the most
//		nextFf := sffs[index]
//
//		remainingSizeWithoutNext, errWithout := calculateWorstCaseSffsSize(
//			threshold, sffs, index+1)
//		sizeWithoutNext := nextFf.omitSize + remainingSizeWithoutNext
//
//		// if sub-fulfillment is not fulfilled, we can only do without
//		if !nextFf.isFulfilled {
//			if errWithout != nil {
//				return 0, calculateWorstCaseSffsSizeError
//			} else {
//				return sizeWithoutNext, nil
//			}
//		}
//
//		remainingSizeWithNext, errWith := calculateWorstCaseSffsSize(
//			subOrZero(threshold, nextFf.weight), sffs, index+1)
//		sizeWithNext := nextFf.size + remainingSizeWithNext
//
//		if errWith != nil && errWithout != nil {
//			return 0, calculateWorstCaseSffsSizeError
//		} else if errWith != nil {
//			return sizeWithoutNext, nil
//		} else if errWithout != nil {
//			return sizeWithNext, nil
//		} else {
//			return maxUint32(sizeWithNext, sizeWithoutNext), nil
//		}
//	} else {
//		return 0, calculateWorstCaseSffsSizeError
//	}
//}
