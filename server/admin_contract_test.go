package server

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

// The admin API has three sides: these handlers, the examples in
// docs/admin-api.md, and the app's decoders. The handlers' JSON must have
// the examples' keys, and the app decodes the examples (ApiExamples.elm,
// written here), so a name changed on one side alone fails.
const elmExamples = "../elm/admin/tests/ApiExamples.elm"

// docExamples is the first JSON example under each heading of
// docs/admin-api.md, by heading, with the placeholders the document puts
// for nested types filled in.
func docExamples(t *testing.T) map[string]string {
	t.Helper()

	doc, err := os.ReadFile("../docs/admin-api.md")
	require.NoError(t, err)

	block := regexp.MustCompile("(?s)```json\n(.*?)```")
	out := map[string]string{}

	for _, sec := range regexp.MustCompile(`(?m)^#+ `).Split(string(doc), -1) {
		heading, body, _ := strings.Cut(sec, "\n")
		if m := block.FindStringSubmatch(body); m != nil {
			out[heading] = m[1]
		}
	}

	out["Photo"] = strings.Replace(out["Photo"], `"status": PhotoStatus`, `"status": `+out["PhotoStatus"], 1)
	out["Event"] = strings.Replace(out["Event"], `{ "...": "Photo" }`, out["Photo"], 1)

	for _, name := range []string{"Photo", "PhotoStatus", "Photographer", "Event", "EventInput"} {
		require.True(t, json.Valid([]byte(out[name])), "the %s example is not JSON:\n%s", name, out[name])
	}

	return out
}

// keys lists every object key in v by its path, array elements as "[]".
func keys(v any) []string {
	out := map[string]bool{}

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				out[prefix+k] = true
				walk(prefix+k+".", x)
			}
		case []any:
			for _, x := range v {
				walk(prefix+"[].", x)
			}
		}
	}
	walk("", v)

	return slices.Sorted(maps.Keys(out))
}

func keysOf(t *testing.T, s string) []string {
	t.Helper()

	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))

	return keys(v)
}

func TestAdminJSONMatchesTheDocs(t *testing.T) {
	ex := docExamples(t)
	a := newAdmin(t, false)
	ctx := t.Context()

	ola, err := a.store.CreatePhotographer(ctx, site.Photographer{Name: "Ola Nordmann", URL: "https://example.no"})
	require.NoError(t, err)

	main := a.storedPhoto(t, 'a')
	in := julemarked(main)
	in["mainPhoto"].(map[string]any)["photographerId"] = ola.ID //nolint:forcetypeassert // ref builds it

	rec := a.do(t, "POST", "/admin/api/events", req{body: in})
	require.Equal(t, 201, rec.Code, rec.Body.String())

	var event map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &event))

	photo := event["mainPhoto"].(map[string]any) //nolint:forcetypeassert // the event has one

	require.Equal(t, keysOf(t, ex["Event"]), keys(event), "Event")
	require.Equal(t, keysOf(t, ex["Photo"]), keys(photo), "Photo")
	require.Equal(t, keysOf(t, ex["PhotoStatus"]), keys(photo["status"]), "PhotoStatus")
	require.Equal(t, keysOf(t, ex["Photographer"]), keys(photo["photographer"]), "Photographer")

	input, err := json.Marshal(eventInput{MainPhoto: &photoRefJSON{}, ExtraPhotos: []photoRefJSON{{}}})
	require.NoError(t, err)
	require.Equal(t, keysOf(t, ex["EventInput"]), keysOf(t, string(input)), "EventInput")

	elm := elmModule(t, ex)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(elmExamples, []byte(elm), 0o600))
		return
	}

	got, err := os.ReadFile(elmExamples)
	require.NoError(t, err)
	require.Equal(t, elm, string(got), "%s is behind docs/admin-api.md: UPDATE_GOLDEN=1 go test ./server", elmExamples)
}

// elmModule is ApiExamples.elm: the examples as Elm strings, laid out as
// elm-format leaves them.
func elmModule(t *testing.T, ex map[string]string) string {
	t.Helper()

	var b strings.Builder

	b.WriteString("module ApiExamples exposing (event, eventInput, photo)\n\n" +
		"{-| The JSON examples of docs/admin-api.md, which the app must decode. The\n" +
		"server's tests write this file: `UPDATE_GOLDEN=1 go test ./server`.\n-}\n")

	for _, name := range []string{"event", "eventInput", "photo"} {
		var c bytes.Buffer
		require.NoError(t, json.Compact(&c, []byte(ex[strings.ToUpper(name[:1])+name[1:]])))

		b.WriteString("\n\n" + name + " : String\n" + name + " =\n    \"\"\"" +
			strings.ReplaceAll(c.String(), `\`, `\\`) + "\"\"\"\n")
	}

	return b.String()
}
