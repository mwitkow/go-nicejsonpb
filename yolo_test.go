package nicejsonpb_test

import (
	"testing"

	"github.com/mwitkow/go-nicejsonpb"
	"github.com/mwitkow/go-nicejsonpb/test"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal_FindsErrorsInArrays(t *testing.T) {
	input := `{"someIntRep": [2,5,1,"sdas",2.1]}`
	stuff := &validatortest.ValidatorMessage3{}
	err := nicejsonpb.UnmarshalString(input, stuff)
	require.EqualError(t, err, "unparsable field SomeIntRep.[3]: json: cannot unmarshal string into Go value of type uint32")
}

func TestUnmarshal_HandlesGoodFormattingOfInt64AsString(t *testing.T) {
	input := `{"someEmbedded": {"someValue": "should_be_int"}}`
	stuff := &validatortest.ValidatorMessage3{}
	err := nicejsonpb.UnmarshalString(input, stuff)
	require.EqualError(t, err, "unparsable field SomeEmbedded.SomeValue: invalid character 's' looking for beginning of value while looking for an integer in a string")
}

func TestUnmarshal_FindsErrorsInNested(t *testing.T) {
	input := `{"someEmbedded": {"identifier": 3.1}}`
	stuff := &validatortest.ValidatorMessage3{}
	err := nicejsonpb.UnmarshalString(input, stuff)
	require.EqualError(t, err, "unparsable field SomeEmbedded.Identifier: json: cannot unmarshal number into Go value of type string")
}

func TestUnmarshal_RemapsRawMessageToRealArrayType(t *testing.T) {
	input := `{"someIntRep": "not_an_array"}`
	stuff := &validatortest.ValidatorMessage3{}
	err := nicejsonpb.UnmarshalString(input, stuff)
	require.EqualError(t, err, "unparsable field SomeIntRep: json: cannot unmarshal string into Go value of type []uint32")
}

func TestUnmarshal_UnknownFieldErrors(t *testing.T) {
	input := `{"someEmbedded": {"someValue": 3, "someUnknown": 1, "anotherUnknown": "foo"}}`
	stuff := &validatortest.ValidatorMessage3{}
	err := nicejsonpb.UnmarshalString(input, stuff)
	require.EqualError(t, err, "unparsable field SomeEmbedded: fields [someUnknown anotherUnknown] do not exist in set of known fields [identifier someValue]")
}
