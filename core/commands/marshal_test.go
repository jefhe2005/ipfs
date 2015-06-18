package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	cmds "github.com/ipfs/go-ipfs/commands"
)

func TestCommandMarshaling(t *testing.T) {
	rsrc := rand.NewSource(1337)
	r := rand.New(rsrc)

	err := checkCommand(t, r, "ipfs", Root)
	if err != nil {
		t.Fatal(err)
	}
}

func checkCommand(t *testing.T, r *rand.Rand, name string, cmd *cmds.Command) error {
	if cmd.Type != nil {
		err := checkTypeMarshal(t, r, cmd.Type)
		if err != nil {
			return fmt.Errorf("%s failed: %s", name, err)
		}
	}

	for cname, child := range cmd.Subcommands {
		err := checkCommand(t, r, name+" "+cname, child)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkTypeMarshal(t *testing.T, r *rand.Rand, typ interface{}) error {
	ourType := reflect.TypeOf(typ)
	objval, ok := quick.Value(ourType, r)
	if !ok {
		t.Log("unable to generate value for: ", reflect.TypeOf(typ))
		return nil
	}
	obj := objval.Interface()

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(obj)
	if err != nil {
		return err
	}

	outv := reflect.New(ourType)
	err = json.NewDecoder(buf).Decode(outv.Interface())
	if err != nil {
		return err
	}
	out := outv.Elem().Interface()

	if !reflect.DeepEqual(obj, out) {
		return fmt.Errorf("objects were not equal after marshaling")
	}

	return nil
}

// generator funcs for difficult types

func (c Command) Generate(r *rand.Rand, size int) reflect.Value {
	m := Command{}
	rs, _ := quick.Value(reflect.TypeOf(""), r)
	m.Name = rs.Interface().(string)

	max := r.Intn(20)
	for i := 0; i < max; i++ {
		sub := Command{}
		rs, _ := quick.Value(reflect.TypeOf(""), r)
		sub.Name = rs.Interface().(string)
		m.Subcommands = append(m.Subcommands, sub)
	}
	return reflect.ValueOf(m)
}
