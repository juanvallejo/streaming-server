package client

import "reflect"

// ResponseFromClientData receives a map of keys to empty interface{}
// and returns a *client.Response object with fields set to values
// from the received map, for every field whose name or json tag value
// matched the corresponding map key.
//
// Example:
//
// data = map[string]interface{}{
//   "commonkey1": "val",
//   "commonkey2": 12345,
//   "uniquekey1": "bob",
// }
//
// Output:
//
// Response{"val" 12345}
func ResponseFromClientData(data map[string]interface{}) Response {
	x := Response{}
	v := reflect.ValueOf(x)

	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		tagVal, ok := f.Tag.Lookup("json")
		if !ok {
			tagVal = f.Name
		}

		if dataVal, ok := data[tagVal]; ok {
			if f.Type == reflect.TypeOf(dataVal) {
				fv := reflect.ValueOf(&x).Elem().Field(i)
				val := reflect.ValueOf(dataVal)

				fv.Set(val)
			}
		}
	}

	return x
}
