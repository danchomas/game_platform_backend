package llm

import (
	"errors"
	"strings"
	"testing"
)

const patchSrc = `<script>
function loop() {
    ctx.ststroke();
    requestAnimationFrame(loop);
}
let speed = 5;
</script>`

func TestParseAndApplyEdits(t *testing.T) {
	raw := `<reply>Исправил опечатку и скорость</reply>
<patch>
<<<<<<< SEARCH
    ctx.ststroke();
=======
    ctx.stroke();
>>>>>>> REPLACE
<<<<<<< SEARCH
let speed = 5;
=======
let speed = 3;
>>>>>>> REPLACE
</patch>`
	reply, edits, err := ParseEdits(raw)
	if err != nil || reply != "Исправил опечатку и скорость" || len(edits) != 2 {
		t.Fatalf("reply=%q edits=%d err=%v", reply, len(edits), err)
	}
	out, err := ApplyEdits(patchSrc, edits)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "    ctx.stroke();") || !strings.Contains(out, "let speed = 3;") || strings.Contains(out, "ststroke") {
		t.Fatalf("not applied:\n%s", out)
	}
}

func TestApplyEditsToleratesIndentation(t *testing.T) {
	out, err := ApplyEdits(patchSrc, []Edit{{
		Search:  "function loop() {\n  ctx.ststroke();", // отступ сбит
		Replace: "function loop() {\n    ctx.stroke();",
	}})
	if err != nil || !strings.Contains(out, "ctx.stroke();") || strings.Contains(out, "ststroke") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestApplyEditsRejectsMissingAndAmbiguous(t *testing.T) {
	if _, err := ApplyEdits(patchSrc, []Edit{{Search: "nothing like this", Replace: "x"}}); err == nil {
		t.Error("missing fragment must fail")
	}
	if _, err := ApplyEdits("a();\nb();\na();", []Edit{{Search: "a();", Replace: "c();"}}); err == nil {
		t.Error("ambiguous fragment must fail")
	}
	if _, _, err := ParseEdits("<reply>ok</reply>"); !errors.Is(err, ErrNoEdits) {
		t.Errorf("no edits: %v", err)
	}
	if _, _, err := ParseEdits("<<<<<<< SEARCH\na\n=======\nb"); err == nil {
		t.Error("unterminated edit must fail")
	}
}
