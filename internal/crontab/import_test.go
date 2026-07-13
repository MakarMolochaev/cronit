package crontab

import "testing"

const sampleCrontab = `MAILTO=admin@example.com

# backup home dir
@daily /usr/local/bin/backup.sh --home
*/5 * * * *	curl -fsS https://example.com/ping
# cronit:id=abc12
*/5 * * * * /usr/local/bin/cronit start abc12 "echo hi"
0~30 * * * * /usr/bin/random-thing
0 9 * * 1-5 echo "hello %USER%"
* * * *
0 3 * * * date +\%Y-\%m-\%d
`

func TestParseForeign(t *testing.T) {
	entries := parseForeign(sampleCrontab)
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries, got %d: %+v", len(entries), entries)
	}

	e := entries[0]
	if e.Schedule != "@daily" || e.Command != "/usr/local/bin/backup.sh --home" {
		t.Errorf("entry 0 parsed wrong: %+v", e)
	}
	if e.Name != "backup home dir" || e.CommentLine != 2 || e.LineNo != 3 {
		t.Errorf("entry 0 comment/name wrong: %+v", e)
	}
	if e.Err != "" {
		t.Errorf("entry 0 unexpected err: %s", e.Err)
	}

	e = entries[1]
	if e.Schedule != "*/5 * * * *" || e.Command != "curl -fsS https://example.com/ping" {
		t.Errorf("entry 1 parsed wrong: %+v", e)
	}
	if e.Name != "" || e.CommentLine != -1 {
		t.Errorf("entry 1 should have no comment: %+v", e)
	}

	if entries[2].Err == "" {
		t.Errorf("entry 2 (0~30) should fail to parse: %+v", entries[2])
	}
	if entries[3].Err == "" {
		t.Errorf("entry 3 (%%) should be rejected: %+v", entries[3])
	}
	if entries[4].Err != "missing command" {
		t.Errorf("entry 4 should be missing command: %+v", entries[4])
	}

	e = entries[5]
	if e.Err != "" || e.Command != "date +%Y-%m-%d" {
		t.Errorf("entry 5 should unescape %%: %+v", e)
	}
}

func TestUnescapePercent(t *testing.T) {
	if got, ok := unescapePercent(`echo 50\% done`); !ok || got != "echo 50% done" {
		t.Errorf(`escaped %% should be accepted, got %q ok=%v`, got, ok)
	}
	if _, ok := unescapePercent("echo 100%"); ok {
		t.Error("unescaped % should be rejected")
	}
	if got, ok := unescapePercent("plain command"); !ok || got != "plain command" {
		t.Errorf("plain command mangled: %q ok=%v", got, ok)
	}
}

func TestParseForeignEmpty(t *testing.T) {
	if got := parseForeign(""); got != nil {
		t.Errorf("empty crontab should yield nil, got %+v", got)
	}
	if got := parseForeign("MAILTO=x\n# just a comment\n"); got != nil {
		t.Errorf("env+comment only should yield nil, got %+v", got)
	}
}

func TestParseForeignSkipsManaged(t *testing.T) {
	content := "# cronit:id=xyz99\n0 0 * * * /bin/managed\n0 1 * * * /bin/free\n"
	entries := parseForeign(content)
	if len(entries) != 1 || entries[0].Command != "/bin/free" {
		t.Fatalf("expected only the unmanaged entry, got %+v", entries)
	}
}

func TestCutFields(t *testing.T) {
	head, rest := cutFields("*/5  * *\t* *   echo  hi there", 5)
	if head != "*/5 * * * *" || rest != "echo  hi there" {
		t.Errorf("cutFields normalized wrong: head=%q rest=%q", head, rest)
	}
	head, rest = cutFields("@daily backup.sh", 1)
	if head != "@daily" || rest != "backup.sh" {
		t.Errorf("cutFields @ wrong: head=%q rest=%q", head, rest)
	}
	head, rest = cutFields("* * * *", 5)
	if head != "* * * *" || rest != "" {
		t.Errorf("cutFields short line wrong: head=%q rest=%q", head, rest)
	}
}
