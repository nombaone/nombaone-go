package nombaone

import (
	"encoding/json"
	"testing"
)

func TestParams_OmitSetAndNull(t *testing.T) {
	type body struct {
		Name  *string           `json:"name,omitempty"`
		Phone *Optional[string] `json:"phone,omitempty"`
	}

	cases := []struct {
		name string
		in   body
		want string
	}{
		{"both unset omit the fields", body{}, `{}`},
		{"a set scalar is sent", body{Name: String("Ada")}, `{"name":"Ada"}`},
		{"a set nullable carries its value", body{Phone: Set("+2348012345678")}, `{"phone":"+2348012345678"}`},
		{"a nulled field is sent as JSON null", body{Phone: Null[string]()}, `{"phone":null}`},
		{
			"name set, phone cleared",
			body{Name: String("Ada"), Phone: Null[string]()},
			`{"name":"Ada","phone":null}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParams_PointerHelpers(t *testing.T) {
	if got := String("x"); got == nil || *got != "x" {
		t.Errorf("String")
	}
	if got := Int(7); got == nil || *got != 7 {
		t.Errorf("Int")
	}
	if got := Int64(250_000); got == nil || *got != 250_000 {
		t.Errorf("Int64")
	}
	if got := Bool(true); got == nil || *got != true {
		t.Errorf("Bool")
	}
}
