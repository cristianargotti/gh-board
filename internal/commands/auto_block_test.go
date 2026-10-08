package commands

import "testing"

var autoBlockText = []byte("## Board\n\nRead the skill.\n")

var autoUpsertCases = []struct {
	name string
	text string
	want string
}{
	{"empty file", "", string(autoBlock(autoBlockText))},
	{"blank file", "\n\n", string(autoBlock(autoBlockText))},
	{"append with newline", "# Team\n", "# Team\n\n" + string(autoBlock(autoBlockText))},
	{"append without newline", "# Team", "# Team\n\n" + string(autoBlock(autoBlockText))},
	{
		"replace",
		"# Team\n\n" + autoBlockBegin + "\nold\n" + autoBlockEnd + "\n\n# After\n",
		"# Team\n\n" + string(autoBlock(autoBlockText)) + "\n# After\n",
	},
	{
		"damaged block",
		"# Team\n\n" + autoBlockBegin + "\nold without end\n",
		"# Team\n\n" + string(autoBlock(autoBlockText)),
	},
	{
		"end marker at end of file",
		autoBlockBegin + "\nold\n" + autoBlockEnd,
		string(autoBlock(autoBlockText)),
	},
}

func TestAutoBlockBodyUnwrapsMarkers(t *testing.T) {
	wrapped := autoBlock(autoBlockText)
	if got := string(autoUpsertBlock(nil, wrapped)); got != string(wrapped) {
		t.Fatalf("wrapped asset =\n%s", got)
	}
	if got := string(autoBlockBody([]byte("plain\n"))); got != "plain\n" {
		t.Fatalf("plain = %q", got)
	}
	if got := string(autoBlockBody([]byte(autoBlockEnd + "\n" + autoBlockBegin))); got != autoBlockEnd+"\n"+autoBlockBegin {
		t.Fatalf("reversed markers = %q", got)
	}
}

func TestAutoUpsertBlock(t *testing.T) {
	for _, tc := range autoUpsertCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(autoUpsertBlock([]byte(tc.text), autoBlockText)); got != tc.want {
				t.Fatalf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

var autoRemoveCases = []struct {
	name string
	text string
	want string
}{
	{"absent", "# Team\n", "# Team\n"},
	{"only block", string(autoBlock(autoBlockText)), ""},
	{"after text", "# Team\n\n" + string(autoBlock(autoBlockText)), "# Team\n"},
	{"between", "# Team\n\n" + string(autoBlock(autoBlockText)) + "\n# After\n", "# Team\n\n# After\n"},
}

func TestAutoRemoveBlock(t *testing.T) {
	for _, tc := range autoRemoveCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(autoRemoveBlock([]byte(tc.text))); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if got := string(autoBlock([]byte("no newline"))); got != autoBlockBegin+"\nno newline\n"+autoBlockEnd+"\n" {
		t.Fatalf("block = %q", got)
	}
}
