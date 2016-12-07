// // Go support for Protocol Buffers - Google's data interchange format
// //
// // Copyright 2015 The Go Authors.  All rights reserved.
// // https://github.com/golang/protobuf
// //
// // Redistribution and use in source and binary forms, with or without
// // modification, are permitted provided that the following conditions are
// // met:
// //
// //     * Redistributions of source code must retain the above copyright
// // notice, this list of conditions and the following disclaimer.
// //     * Redistributions in binary form must reproduce the above
// // copyright notice, this list of conditions and the following disclaimer
// // in the documentation and/or other materials provided with the
// // distribution.
// //     * Neither the name of Google Inc. nor the names of its
// // contributors may be used to endorse or promote products derived from
// // this software without specific prior written permission.
// //
// // THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// // "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// // LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// // A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// // OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// // SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// // LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// // DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// // THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// // (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// // OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package nicejsonpb

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
)

// Unmarshaler is a configurable object for converting from a JSON
// representation to a protocol buffer object.
type Unmarshaler struct {
	// Whether to allow messages to contain unknown fields, as opposed to
	// failing to unmarshal.
	AllowUnknownFields bool
}

// UnmarshalNext unmarshals the next protocol buffer from a JSON object stream.
// This function is lenient and will decode any options permutations of the
// related Marshaler.
func (u *Unmarshaler) UnmarshalNext(dec *json.Decoder, pb proto.Message) error {
	inputValue := json.RawMessage{}
	if err := dec.Decode(&inputValue); err != nil {
		return err
	}
	return u.unmarshalValue(reflect.ValueOf(pb).Elem(), inputValue, nil)
}

// Unmarshal unmarshals a JSON object stream into a protocol
// buffer. This function is lenient and will decode any options
// permutations of the related Marshaler.
func (u *Unmarshaler) Unmarshal(r io.Reader, pb proto.Message) error {
	dec := json.NewDecoder(r)
	return u.UnmarshalNext(dec, pb)
}

// UnmarshalNext unmarshals the next protocol buffer from a JSON object stream.
// This function is lenient and will decode any options permutations of the
// related Marshaler.
func UnmarshalNext(dec *json.Decoder, pb proto.Message) error {
	return new(Unmarshaler).UnmarshalNext(dec, pb)
}

// Unmarshal unmarshals a JSON object stream into a protocol
// buffer. This function is lenient and will decode any options
// permutations of the related Marshaler.
func Unmarshal(r io.Reader, pb proto.Message) error {
	return new(Unmarshaler).Unmarshal(r, pb)
}

// UnmarshalString will populate the fields of a protocol buffer based
// on a JSON string. This function is lenient and will decode any options
// permutations of the related Marshaler.
func UnmarshalString(str string, pb proto.Message) error {
	return new(Unmarshaler).Unmarshal(strings.NewReader(str), pb)
}

// unmarshalValue converts/copies a value into the target.
// prop may be nil.
func (u *Unmarshaler) unmarshalValue(target reflect.Value, inputValue json.RawMessage, prop *proto.Properties) error {
	targetType := target.Type()

	// Allocate memory for pointer fields.
	if targetType.Kind() == reflect.Ptr {
		target.Set(reflect.New(targetType.Elem()))
		return u.unmarshalValue(target.Elem(), inputValue, prop)
	}

	// Handle well-known types.
	type wkt interface {
		XXX_WellKnownType() string
	}
	if wkt, ok := target.Addr().Interface().(wkt); ok {
		switch wkt.XXX_WellKnownType() {
		case "DoubleValue", "FloatValue", "Int64Value", "UInt64Value",
			"Int32Value", "UInt32Value", "BoolValue", "StringValue", "BytesValue":
			// "Wrappers use the same representation in JSON
			//  as the wrapped primitive type, except that null is allowed."
			// encoding/json will turn JSON `null` into Go `nil`,
			// so we don't have to do any extra work.
			return u.unmarshalValue(target.Field(0), inputValue, prop)
		case "Any":
			return fmt.Errorf("unmarshaling Any not supported yet")
		case "Duration":
			unq, err := strconv.Unquote(string(inputValue))
			if err != nil {
				return err
			}
			d, err := time.ParseDuration(unq)
			if err != nil {
				return fmt.Errorf("bad Duration: %v", err)
			}
			ns := d.Nanoseconds()
			s := ns / 1e9
			ns %= 1e9
			target.Field(0).SetInt(s)
			target.Field(1).SetInt(ns)
			return nil
		case "Timestamp":
			unq, err := strconv.Unquote(string(inputValue))
			if err != nil {
				return err
			}
			t, err := time.Parse(time.RFC3339Nano, unq)
			if err != nil {
				return fmt.Errorf("bad Timestamp: %v", err)
			}
			ns := t.UnixNano()
			s := ns / 1e9
			ns %= 1e9
			target.Field(0).SetInt(s)
			target.Field(1).SetInt(ns)
			return nil
		}
	}

	// Handle enums, which have an underlying type of int32,
	// and may appear as strings.
	// The case of an enum appearing as a number is handled
	// at the bottom of this function.
	if inputValue[0] == '"' && prop != nil && prop.Enum != "" {
		vmap := proto.EnumValueMap(prop.Enum)
		// Don't need to do unquoting; valid enum names
		// are from a limited character set.
		s := inputValue[1 : len(inputValue)-1]
		n, ok := vmap[string(s)]
		if !ok {
			return fmt.Errorf("unknown value '%q' for enum %s", s, prop.Enum)
		}
		if target.Kind() == reflect.Ptr { // proto2
			target.Set(reflect.New(targetType.Elem()))
			target = target.Elem()
		}
		target.SetInt(int64(n))
		return nil
	}

	// Handle nested messages.
	if targetType.Kind() == reflect.Struct {
		var jsonFields map[string]json.RawMessage
		if err := json.Unmarshal(inputValue, &jsonFields); err != nil {
			return correctJsonType(err, targetType)
		}

		consumeField := func(prop *proto.Properties) (json.RawMessage, bool) {
			// Be liberal in what names we accept; both orig_name and camelName are okay.
			fieldNames := acceptedJSONFieldNames(prop)

			vOrig, okOrig := jsonFields[fieldNames.orig]
			vCamel, okCamel := jsonFields[fieldNames.camel]
			if !okOrig && !okCamel {
				return nil, false
			}
			// If, for some reason, both are present in the data, favour the camelName.
			var raw json.RawMessage
			if okOrig {
				raw = vOrig
				delete(jsonFields, fieldNames.orig)
			}
			if okCamel {
				raw = vCamel
				delete(jsonFields, fieldNames.camel)
			}
			return raw, true
		}

		sprops := proto.GetProperties(targetType)
		for i := 0; i < target.NumField(); i++ {
			ft := target.Type().Field(i)
			if strings.HasPrefix(ft.Name, "XXX_") {
				continue
			}

			valueForField, ok := consumeField(sprops.Prop[i])
			if !ok {
				continue
			}

			if err := u.unmarshalValue(target.Field(i), valueForField, sprops.Prop[i]); err != nil {
				return FieldError(sprops.Prop[i].Name, err)
			}
		}
		// Check for any oneof fields.
		if len(jsonFields) > 0 {
			for _, oop := range sprops.OneofTypes {
				raw, ok := consumeField(oop.Prop)
				if !ok {
					continue
				}
				nv := reflect.New(oop.Type.Elem())
				target.Field(oop.Field).Set(nv)
				if err := u.unmarshalValue(nv.Elem().Field(0), raw, oop.Prop); err != nil {
					return FieldError(oop.Prop.Name, err)
				}
			}
		}
		if !u.AllowUnknownFields && len(jsonFields) > 0 {
			return getFieldMismatchError(jsonFields, sprops)
		}
		return nil
	}

	// Handle arrays (which aren't encoded bytes)
	if targetType.Kind() == reflect.Slice && targetType.Elem().Kind() != reflect.Uint8 {
		var slc []json.RawMessage
		if err := json.Unmarshal(inputValue, &slc); err != nil {
			return correctJsonType(err, targetType)
		}
		len := len(slc)
		target.Set(reflect.MakeSlice(targetType, len, len))
		for i := 0; i < len; i++ {
			if err := u.unmarshalValue(target.Index(i), slc[i], prop); err != nil {
				return FieldError(fmt.Sprintf("[%d]", i), err)
			}
		}
		return nil
	}

	// Handle maps (whose keys are always strings)
	if targetType.Kind() == reflect.Map {
		var mp map[string]json.RawMessage
		if err := json.Unmarshal(inputValue, &mp); err != nil {
			return err
		}
		target.Set(reflect.MakeMap(targetType))
		var keyprop, valprop *proto.Properties
		if prop != nil {
			// These could still be nil if the protobuf metadata is broken somehow.
			// TODO: This won't work because the fields are unexported.
			// We should probably just reparse them.
			//keyprop, valprop = prop.mkeyprop, prop.mvalprop
		}
		for ks, raw := range mp {
			// Unmarshal map key. The core json library already decoded the key into a
			// string, so we handle that specially. Other types were quoted post-serialization.
			var k reflect.Value
			if targetType.Key().Kind() == reflect.String {
				k = reflect.ValueOf(ks)
			} else {
				k = reflect.New(targetType.Key()).Elem()
				if err := u.unmarshalValue(k, json.RawMessage(ks), keyprop); err != nil {
					return FieldError(fmt.Sprintf("['%s']key", ks), err)
				}
			}

			// Unmarshal map value.
			v := reflect.New(targetType.Elem()).Elem()
			if err := u.unmarshalValue(v, raw, valprop); err != nil {
				return FieldError(fmt.Sprintf("['%s']value", ks), err)
			}
			target.SetMapIndex(k, v)
		}
		return nil
	}

	// 64-bit integers can be encoded as strings. In this case we drop
	// the quotes and proceed as normal.
	isNum := targetType.Kind() == reflect.Int64 || targetType.Kind() == reflect.Uint64
	if isNum && strings.HasPrefix(string(inputValue), `"`) {
		inputValue = inputValue[1 : len(inputValue)-1]
		if err := json.Unmarshal(inputValue, target.Addr().Interface()); err != nil {
			return fmt.Errorf("%v while looking for an integer in a string", err)
		}
		return nil
	} else {
		// Use the encoding/json for parsing other value types.
		return json.Unmarshal(inputValue, target.Addr().Interface())
	}
}

// jsonProperties returns parsed proto.Properties for the field and corrects JSONName attribute.
func jsonProperties(f reflect.StructField, origName bool) *proto.Properties {
	var prop proto.Properties
	prop.Init(f.Type, f.Name, f.Tag.Get("protobuf"), &f)
	if origName || prop.JSONName == "" {
		prop.JSONName = prop.OrigName
	}
	return &prop
}

type fieldNames struct {
	orig, camel string
}

func acceptedJSONFieldNames(prop *proto.Properties) fieldNames {
	opts := fieldNames{orig: prop.OrigName, camel: prop.OrigName}
	if prop.JSONName != "" {
		opts.camel = prop.JSONName
	}
	return opts
}
