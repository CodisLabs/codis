// Package binding transforms a raw request into a struct
// ready to be used your application. It can also perform
// validation on the data and handle errors.
package binding

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
)

/*
	For the land of Middle-ware Earth:
		One func to rule them all,
		One func to find them,
		One func to bring them all,
		And in this package BIND them.
*/

// Bind wraps up the functionality of the Form and Json middleware
// according to the Content-Type and verb of the request.
// A Content-Type is required for POST and PUT requests.
// Bind invokes the ErrorHandler middleware to bail out if errors
// occurred. If you want to perform your own error handling, use
// Form or Json middleware directly. An interface pointer can
// be added as a second argument in order to map the struct to
// a specific interface.
func Bind(obj interface{}, ifacePtr ...interface{}) martini.Handler {
	return func(context martini.Context, req *http.Request) {
		contentType := req.Header.Get("Content-Type")

		if req.Method == "POST" || req.Method == "PUT" || contentType != "" {
			if strings.Contains(contentType, "form-urlencoded") {
				context.Invoke(Form(obj, ifacePtr...))
			} else if strings.Contains(contentType, "multipart/form-data") {
				context.Invoke(MultipartForm(obj, ifacePtr...))
			} else if strings.Contains(contentType, "json") {
				context.Invoke(Json(obj, ifacePtr...))
			} else {
				var errors Errors
				if contentType == "" {
					errors.Add([]string{}, ContentTypeError, "Empty Content-Type")
				} else {
					errors.Add([]string{}, ContentTypeError, "Unsupported Content-Type")
				}
				context.Map(errors)
			}
		} else {
			context.Invoke(Form(obj, ifacePtr...))
		}

		context.Invoke(ErrorHandler)
	}
}

// Form is middleware to deserialize form-urlencoded data from the request.
// It gets data from the form-urlencoded body, if present, or from the
// query string. It uses the http.Request.ParseForm() method
// to perform deserialization, then reflection is used to map each field
// into the struct with the proper type. Structs with primitive slice types
// (bool, float, int, string) can support deserialization of repeated form
// keys, for example: key=val1&key=val2&key=val3
// An interface pointer can be added as a second argument in order
// to map the struct to a specific interface.
func Form(formStruct interface{}, ifacePtr ...interface{}) martini.Handler {
	return func(context martini.Context, req *http.Request) {
		var errors Errors

		ensureNotPointer(formStruct)
		formStruct := reflect.New(reflect.TypeOf(formStruct))
		parseErr := req.ParseForm()

		// Format validation of the request body or the URL would add considerable overhead,
		// and ParseForm does not complain when URL encoding is off.
		// Because an empty request body or url can also mean absence of all needed values,
		// it is not in all cases a bad request, so let's return 422.
		if parseErr != nil {
			errors.Add([]string{}, DeserializationError, parseErr.Error())
		}
		mapForm(formStruct, req.Form, nil, errors)
		validateAndMap(formStruct, context, errors, ifacePtr...)
	}
}

// MultipartForm works much like Form, except it can parse multipart forms
// and handle file uploads. Like the other deserialization middleware handlers,
// you can pass in an interface to make the interface available for injection
// into other handlers later.
func MultipartForm(formStruct interface{}, ifacePtr ...interface{}) martini.Handler {
	return func(context martini.Context, req *http.Request) {
		var errors Errors

		ensureNotPointer(formStruct)
		formStruct := reflect.New(reflect.TypeOf(formStruct))

		// This if check is necessary due to https://github.com/martini-contrib/csrf/issues/6
		if req.MultipartForm == nil {
			// Workaround for multipart forms returning nil instead of an error
			// when content is not multipart; see https://code.google.com/p/go/issues/detail?id=6334
			if multipartReader, err := req.MultipartReader(); err != nil {
				// TODO: Cover this and the next error check with tests
				errors.Add([]string{}, DeserializationError, err.Error())
			} else {
				form, parseErr := multipartReader.ReadForm(MaxMemory)
				if parseErr != nil {
					errors.Add([]string{}, DeserializationError, parseErr.Error())
				}
				req.MultipartForm = form
			}
		}

		mapForm(formStruct, req.MultipartForm.Value, req.MultipartForm.File, errors)
		validateAndMap(formStruct, context, errors, ifacePtr...)
	}

}

// Json is middleware to deserialize a JSON payload from the request
// into the struct that is passed in. The resulting struct is then
// validated, but no error handling is actually performed here.
// An interface pointer can be added as a second argument in order
// to map the struct to a specific interface.
func Json(jsonStruct interface{}, ifacePtr ...interface{}) martini.Handler {
	return func(context martini.Context, req *http.Request) {
		var errors Errors

		ensureNotPointer(jsonStruct)

		jsonStruct := reflect.New(reflect.TypeOf(jsonStruct))

		if req.Body != nil {
			defer req.Body.Close()
			err := json.NewDecoder(req.Body).Decode(jsonStruct.Interface())
			if err != nil && err != io.EOF {
				errors.Add([]string{}, DeserializationError, err.Error())
			}
		}

		validateAndMap(jsonStruct, context, errors, ifacePtr...)
	}
}

// Validate is middleware to enforce required fields. If the struct
// passed in implements Validator, then the user-defined Validate method
// is executed, and its errors are mapped to the context. This middleware
// performs no error handling: it merely detects errors and maps them.
func Validate(obj interface{}) martini.Handler {
	return func(context martini.Context, req *http.Request) {
		var errors Errors

		v := reflect.ValueOf(obj)
		k := v.Kind()

		if k == reflect.Interface || k == reflect.Ptr {

			v = v.Elem()
			k = v.Kind()
		}

		if k == reflect.Slice || k == reflect.Array {

			for i := 0; i < v.Len(); i++ {

				e := v.Index(i).Interface()
				errors = validateStruct(errors, e)

				if validator, ok := e.(Validator); ok {
					errors = validator.Validate(errors, req)
				}
			}
		} else {

			errors = validateStruct(errors, obj)

			if validator, ok := obj.(Validator); ok {
				errors = validator.Validate(errors, req)
			}
		}
		context.Map(errors)
	}
}

// Performs required field checking on a struct
func validateStruct(errors Errors, obj interface{}) Errors {
	typ := reflect.TypeOf(obj)
	val := reflect.ValueOf(obj)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip ignored and unexported fields in the struct
		if field.Tag.Get("form") == "-" || !val.Field(i).CanInterface() {
			continue
		}

		fieldValue := val.Field(i).Interface()
		zero := reflect.Zero(field.Type).Interface()

		// Validate nested and embedded structs (if pointer, only do so if not nil)
		if field.Type.Kind() == reflect.Struct ||
			(field.Type.Kind() == reflect.Ptr && !reflect.DeepEqual(zero, fieldValue) &&
				field.Type.Elem().Kind() == reflect.Struct) {
			errors = validateStruct(errors, fieldValue)
		}

		if strings.Index(field.Tag.Get("binding"), "required") > -1 {
			if reflect.DeepEqual(zero, fieldValue) {
				name := field.Name
				if j := field.Tag.Get("json"); j != "" {
					name = j
				} else if f := field.Tag.Get("form"); f != "" {
					name = f
				}
				errors.Add([]string{name}, RequiredError, "Required")
			}
		}
	}
	return errors
}

// Takes values from the form data and puts them into a struct
func mapForm(formStruct reflect.Value, form map[string][]string,
	formfile map[string][]*multipart.FileHeader, errors Errors) {

	if formStruct.Kind() == reflect.Ptr {
		formStruct = formStruct.Elem()
	}
	typ := formStruct.Type()

	for i := 0; i < typ.NumField(); i++ {
		typeField := typ.Field(i)
		structField := formStruct.Field(i)

		if typeField.Type.Kind() == reflect.Ptr && typeField.Anonymous {
			structField.Set(reflect.New(typeField.Type.Elem()))
			mapForm(structField.Elem(), form, formfile, errors)
			if reflect.DeepEqual(structField.Elem().Interface(), reflect.Zero(structField.Elem().Type()).Interface()) {
				structField.Set(reflect.Zero(structField.Type()))
			}
		} else if typeField.Type.Kind() == reflect.Struct {
			mapForm(structField, form, formfile, errors)
		} else if inputFieldName := typeField.Tag.Get("form"); inputFieldName != "" {
			if !structField.CanSet() {
				continue
			}

			inputValue, exists := form[inputFieldName]
			if exists {
				numElems := len(inputValue)
				if structField.Kind() == reflect.Slice && numElems > 0 {
					sliceOf := structField.Type().Elem().Kind()
					slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
					for i := 0; i < numElems; i++ {
						setWithProperType(sliceOf, inputValue[i], slice.Index(i), inputFieldName, errors)
					}
					formStruct.Field(i).Set(slice)
				} else {
					setWithProperType(typeField.Type.Kind(), inputValue[0], structField, inputFieldName, errors)
				}
				continue
			}

			inputFile, exists := formfile[inputFieldName]
			if !exists {
				continue
			}
			fhType := reflect.TypeOf((*multipart.FileHeader)(nil))
			numElems := len(inputFile)
			if structField.Kind() == reflect.Slice && numElems > 0 && structField.Type().Elem() == fhType {
				slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
				for i := 0; i < numElems; i++ {
					slice.Index(i).Set(reflect.ValueOf(inputFile[i]))
				}
				structField.Set(slice)
			} else if structField.Type() == fhType {
				structField.Set(reflect.ValueOf(inputFile[0]))
			}
		}
	}
}

// ErrorHandler simply counts the number of errors in the
// context and, if more than 0, writes a response with an
// error code and a JSON payload describing the errors.
// The response will have a JSON content-type.
// Middleware remaining on the stack will not even see the request
// if, by this point, there are any errors.
// This is a "default" handler, of sorts, and you are
// welcome to use your own instead. The Bind middleware
// invokes this automatically for convenience.
func ErrorHandler(errs Errors, resp http.ResponseWriter) {
	if len(errs) > 0 {
		resp.Header().Set("Content-Type", jsonContentType)
		if errs.Has(DeserializationError) {
			resp.WriteHeader(http.StatusBadRequest)
		} else if errs.Has(ContentTypeError) {
			resp.WriteHeader(http.StatusUnsupportedMediaType)
		} else {
			resp.WriteHeader(StatusUnprocessableEntity)
		}
		errOutput, _ := json.Marshal(errs)
		resp.Write(errOutput)
		return
	}
}

// This sets the value in a struct of an indeterminate type to the
// matching value from the request (via Form middleware) in the
// same type, so that not all deserialized values have to be strings.
// Supported types are string, int, float, and bool.
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value, nameInTag string, errors Errors) {
	switch valueKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val == "" {
			val = "0"
		}
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, TypeError, "Value could not be parsed as integer")
		} else {
			structField.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val == "" {
			val = "0"
		}
		uintVal, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, TypeError, "Value could not be parsed as unsigned integer")
		} else {
			structField.SetUint(uintVal)
		}
	case reflect.Bool:
		if val == "" {
			val = "false"
		}
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			errors.Add([]string{nameInTag}, TypeError, "Value could not be parsed as boolean")
		} else {
			structField.SetBool(boolVal)
		}
	case reflect.Float32:
		if val == "" {
			val = "0.0"
		}
		floatVal, err := strconv.ParseFloat(val, 32)
		if err != nil {
			errors.Add([]string{nameInTag}, TypeError, "Value could not be parsed as 32-bit float")
		} else {
			structField.SetFloat(floatVal)
		}
	case reflect.Float64:
		if val == "" {
			val = "0.0"
		}
		floatVal, err := strconv.ParseFloat(val, 64)
		if err != nil {
			errors.Add([]string{nameInTag}, TypeError, "Value could not be parsed as 64-bit float")
		} else {
			structField.SetFloat(floatVal)
		}
	case reflect.String:
		structField.SetString(val)
	}
}

// Don't pass in pointers to bind to. Can lead to bugs. See:
// https://github.com/codegangsta/martini-contrib/pull/34#issuecomment-29683659
func ensureNotPointer(obj interface{}) {
	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		panic("Pointers are not accepted as binding models")
	}
}

// Performs validation and combines errors from validation
// with errors from deserialization, then maps both the
// resulting struct and the errors to the context.
func validateAndMap(obj reflect.Value, context martini.Context, errors Errors, ifacePtr ...interface{}) {
	context.Invoke(Validate(obj.Interface()))
	errors = append(errors, getErrors(context)...)
	context.Map(errors)
	context.Map(obj.Elem().Interface())
	if len(ifacePtr) > 0 {
		context.MapTo(obj.Elem().Interface(), ifacePtr[0])
	}
}

// getErrors simply gets the errors from the context (it's kind of a chore)
func getErrors(context martini.Context) Errors {
	return context.Get(reflect.TypeOf(Errors{})).Interface().(Errors)
}

type (
	// Implement the Validator interface to handle some rudimentary
	// request validation logic so your application doesn't have to.
	Validator interface {
		// Validate validates that the request is OK. It is recommended
		// that validation be limited to checking values for syntax and
		// semantics, enough to know that you can make sense of the request
		// in your application. For example, you might verify that a credit
		// card number matches a valid pattern, but you probably wouldn't
		// perform an actual credit card authorization here.
		Validate(Errors, *http.Request) Errors
	}
)

var (
	// Maximum amount of memory to use when parsing a multipart form.
	// Set this to whatever value you prefer; default is 10 MB.
	MaxMemory = int64(1024 * 1024 * 10)
)

const (
	jsonContentType           = "application/json; charset=utf-8"
	StatusUnprocessableEntity = 422
)
