package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// dispatcher chama os métodos exportados das structs ligadas, como o Bind do Wails:
// "VaultService.Unlock" com os argumentos em JSON, um por parâmetro.
type dispatcher struct {
	methods map[string]reflect.Value
}

func newDispatcher(bound []any) *dispatcher {
	d := &dispatcher{methods: map[string]reflect.Value{}}
	for _, service := range bound {
		v := reflect.ValueOf(service)
		name := reflect.Indirect(v).Type().Name()
		for i := range v.NumMethod() {
			d.methods[name+"."+v.Type().Method(i).Name] = v.Method(i)
		}
	}
	return d
}

// call devolve o primeiro valor do método ou o erro dele. Pânico vira erro: um método com
// defeito não derruba o app.
func (d *dispatcher) call(name string, rawArgs []json.RawMessage) (result any, err error) {
	method, ok := d.methods[name]
	if !ok {
		return nil, fmt.Errorf("método desconhecido: %s", name)
	}
	t := method.Type()
	if t.IsVariadic() || len(rawArgs) > t.NumIn() {
		return nil, fmt.Errorf("%s: argumentos demais", name)
	}
	args := make([]reflect.Value, t.NumIn())
	for i := range args {
		arg := reflect.New(t.In(i))
		if i < len(rawArgs) && len(rawArgs[i]) > 0 {
			if err := json.Unmarshal(rawArgs[i], arg.Interface()); err != nil {
				return nil, fmt.Errorf("%s: argumento %d inválido", name, i)
			}
		}
		args[i] = arg.Elem()
	}
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, errors.New("pânico no método "+name)
		}
	}()
	out := method.Call(args)
	if n := len(out); n > 0 && t.Out(n-1) == errorType {
		if e, _ := out[n-1].Interface().(error); e != nil {
			return nil, e
		}
		out = out[:n-1]
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0].Interface(), nil
}
