//  Copyright (c) 2025 Metaform Systems, Inc
//
//  This program and the accompanying materials are made available under the
//  terms of the Apache License, Version 2.0 which is available at
//  https://www.apache.org/licenses/LICENSE-2.0
//
//  SPDX-License-Identifier: Apache-2.0
//
//  Contributors:
//       Metaform Systems, Inc. - initial API and implementation
//

package types

import (
	"errors"
	"fmt"
)

var (
	// ErrConflict indicates an object conflict, e.g. when creating an object that already exists
	ErrConflict = NewRecoverableError("conflict")
	// ErrNotFound indicates that a certain object does not exist
	ErrNotFound = NewRecoverableError("not found")
	// ErrInvalidInput Sentinel error to indicate a wrong input, e.g., a string when a number was expected, or an empty string
	ErrInvalidInput = NewRecoverableError("invalid input")
)

type RecoverableError interface {
	error
	IsRecoverable() bool
}

type ClientError interface {
	error
	IsClientError() bool
}

type FatalError interface {
	error
	IsFatal() bool
}

type GeneralRecoverableError struct {
	Message string
	Cause   error
}

func (e GeneralRecoverableError) Error() string       { return e.Message }
func (e GeneralRecoverableError) Unwrap() error       { return e.Cause }
func (e GeneralRecoverableError) IsRecoverable() bool { return true }

type BadRequestError struct {
	Message string
	Cause   error
}

func (e BadRequestError) Error() string       { return e.Message }
func (e BadRequestError) IsClientError() bool { return true }
func (e BadRequestError) Unwrap() error       { return e.Cause }

type SystemError struct {
	Message string
	Cause   error
}

func (e SystemError) Error() string { return e.Message }
func (e SystemError) IsFatal() bool { return true }
func (e SystemError) Unwrap() error { return e.Cause }

func NewRecoverableError(message string, args ...any) error {
	return GeneralRecoverableError{Message: fmt.Sprintf(message, args...)}
}

func NewClientError(message string, args ...any) error {
	return BadRequestError{Message: fmt.Sprintf(message, args...)}
}

func NewFatalError(message string, args ...any) error {
	return SystemError{Message: fmt.Sprintf(message, args...)}
}

func NewRecoverableWrappedError(cause error, message string, args ...any) error {
	formattedMessage := fmt.Sprintf(message, args...)
	if cause != nil {
		formattedMessage = fmt.Sprintf("%s: %s", formattedMessage, cause.Error())
	}
	return GeneralRecoverableError{
		Message: formattedMessage,
		Cause:   cause,
	}
}

func NewClientWrappedError(cause error, message string, args ...any) error {
	formattedMessage := fmt.Sprintf(message, args...)
	if cause != nil {
		formattedMessage = fmt.Sprintf("%s: %s", formattedMessage, cause.Error())
	}
	return BadRequestError{
		Message: formattedMessage,
		Cause:   cause,
	}
}

func NewFatalWrappedError(cause error, message string, args ...any) error {
	formattedMessage := fmt.Sprintf(message, args...)
	if cause != nil {
		formattedMessage = fmt.Sprintf("%s: %s", formattedMessage, cause.Error())
	}
	return SystemError{
		Message: formattedMessage,
		Cause:   cause,
	}
}

func IsRecoverable(err error) bool {
	var recErr RecoverableError
	return errors.As(err, &recErr) && recErr.IsRecoverable()
}

func IsClientError(err error) bool {
	var clientErr ClientError
	return errors.As(err, &clientErr) && clientErr.IsClientError()
}

func IsFatal(err error) bool {
	var fatalErr FatalError
	return errors.As(err, &fatalErr) && fatalErr.IsFatal()
}

func NewValidationError(path string, message string) error {
	return BadRequestError{Message: fmt.Sprintf("Validation error at path [%s]: %s", path, message)}
}
