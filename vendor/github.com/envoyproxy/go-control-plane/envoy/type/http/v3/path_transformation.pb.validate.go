// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: envoy/type/http/v3/path_transformation.proto

package envoy_type_http_v3

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
)

// Validate checks the field values on PathTransformation with the rules
// defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *PathTransformation) Validate() error {
	if m == nil {
		return nil
	}

	for idx, item := range m.GetOperations() {
		_, _ = idx, item

		if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return PathTransformationValidationError{
					field:  fmt.Sprintf("Operations[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	return nil
}

// PathTransformationValidationError is the validation error returned by
// PathTransformation.Validate if the designated constraints aren't met.
type PathTransformationValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e PathTransformationValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e PathTransformationValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e PathTransformationValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e PathTransformationValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e PathTransformationValidationError) ErrorName() string {
	return "PathTransformationValidationError"
}

// Error satisfies the builtin error interface
func (e PathTransformationValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sPathTransformation.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = PathTransformationValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = PathTransformationValidationError{}

// Validate checks the field values on PathTransformation_Operation with the
// rules defined in the proto definition for this message. If any rules are
// violated, an error is returned.
func (m *PathTransformation_Operation) Validate() error {
	if m == nil {
		return nil
	}

	switch m.OperationSpecifier.(type) {

	case *PathTransformation_Operation_NormalizePathRfc_3986:

		if v, ok := interface{}(m.GetNormalizePathRfc_3986()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return PathTransformation_OperationValidationError{
					field:  "NormalizePathRfc_3986",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	case *PathTransformation_Operation_MergeSlashes_:

		if v, ok := interface{}(m.GetMergeSlashes()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return PathTransformation_OperationValidationError{
					field:  "MergeSlashes",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	default:
		return PathTransformation_OperationValidationError{
			field:  "OperationSpecifier",
			reason: "value is required",
		}

	}

	return nil
}

// PathTransformation_OperationValidationError is the validation error returned
// by PathTransformation_Operation.Validate if the designated constraints
// aren't met.
type PathTransformation_OperationValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e PathTransformation_OperationValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e PathTransformation_OperationValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e PathTransformation_OperationValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e PathTransformation_OperationValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e PathTransformation_OperationValidationError) ErrorName() string {
	return "PathTransformation_OperationValidationError"
}

// Error satisfies the builtin error interface
func (e PathTransformation_OperationValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sPathTransformation_Operation.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = PathTransformation_OperationValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = PathTransformation_OperationValidationError{}

// Validate checks the field values on
// PathTransformation_Operation_NormalizePathRFC3986 with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *PathTransformation_Operation_NormalizePathRFC3986) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// PathTransformation_Operation_NormalizePathRFC3986ValidationError is the
// validation error returned by
// PathTransformation_Operation_NormalizePathRFC3986.Validate if the
// designated constraints aren't met.
type PathTransformation_Operation_NormalizePathRFC3986ValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) Field() string {
	return e.field
}

// Reason function returns reason value.
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) Reason() string {
	return e.reason
}

// Cause function returns cause value.
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) Cause() error {
	return e.cause
}

// Key function returns key value.
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) ErrorName() string {
	return "PathTransformation_Operation_NormalizePathRFC3986ValidationError"
}

// Error satisfies the builtin error interface
func (e PathTransformation_Operation_NormalizePathRFC3986ValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sPathTransformation_Operation_NormalizePathRFC3986.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = PathTransformation_Operation_NormalizePathRFC3986ValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = PathTransformation_Operation_NormalizePathRFC3986ValidationError{}

// Validate checks the field values on
// PathTransformation_Operation_MergeSlashes with the rules defined in the
// proto definition for this message. If any rules are violated, an error is returned.
func (m *PathTransformation_Operation_MergeSlashes) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// PathTransformation_Operation_MergeSlashesValidationError is the validation
// error returned by PathTransformation_Operation_MergeSlashes.Validate if the
// designated constraints aren't met.
type PathTransformation_Operation_MergeSlashesValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e PathTransformation_Operation_MergeSlashesValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e PathTransformation_Operation_MergeSlashesValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e PathTransformation_Operation_MergeSlashesValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e PathTransformation_Operation_MergeSlashesValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e PathTransformation_Operation_MergeSlashesValidationError) ErrorName() string {
	return "PathTransformation_Operation_MergeSlashesValidationError"
}

// Error satisfies the builtin error interface
func (e PathTransformation_Operation_MergeSlashesValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sPathTransformation_Operation_MergeSlashes.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = PathTransformation_Operation_MergeSlashesValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = PathTransformation_Operation_MergeSlashesValidationError{}
