package nicejsonpb

import (
	"strings"
	"reflect"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"fmt"
)

type fieldError struct {
	fieldStack []string
	nestedErr  error
}

func (f *fieldError) Error() string {
	return "unparsable field " + strings.Join(f.fieldStack, ".") + ": " + f.nestedErr.Error()
}

// FieldError wraps a given error providing a message call stack.
func FieldError(fieldName string, err error) error {
	if fErr, ok := err.(*fieldError); ok {
		fErr.fieldStack = append([]string{fieldName}, fErr.fieldStack...)
		return err
	}
	return &fieldError{
		fieldStack: []string{fieldName},
		nestedErr:  err,
	}
}

// correctJsonType gets rid of the dredded json.RawMessage errors and casts them to the right type.
func correctJsonType(err error, realType reflect.Type) error {
	if uErr, ok := err.(*json.UnmarshalTypeError); ok {
		uErr.Type = realType
		return uErr
	}
	return err
}

func getFieldMismatchError(remainingFields map[string]json.RawMessage, structProps *proto.StructProperties) error {
	remaining := []string{}
	for k, _ := range remainingFields {
		remaining = append(remaining, k)
	}
	known := []string{}
	for _, prop := range structProps.Prop {
		jsonNames := acceptedJSONFieldNames(prop)
		known = append(known, jsonNames.camel)
	}
	return fmt.Errorf("fields %v do not exist in set of known fields %v", remaining, known)
}