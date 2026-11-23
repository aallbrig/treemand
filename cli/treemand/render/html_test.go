package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/render"
)

func htmlOf(t *testing.T, root *models.Node, edit func(*render.Options)) string {
	t.Helper()
	opts := render.DefaultOptions()
	opts.Output = "html"
	if edit != nil {
		edit(&opts)
	}
	got, err := render.ToString(root, opts)
	if err != nil {
		t.Fatalf("ToString(html): %v", err)
	}
	return got
}

func TestRenderHTML_isOneSelfContainedPage(t *testing.T) {
	got := htmlOf(t, sampleTree(), nil)
	for _, want := range []string{
		"<!doctype html>", `<html lang="en">`, "<title>git command tree</title>",
		`<input id="q" type="search"`, `<label for="q"`, "<details", "<summary",
		"<style>", "<script>", "Content-Security-Policy",
		"commit", "record changes to the repository", "--message", "&lt;name&gt;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("html is missing %q", want)
		}
	}
	// No network: nothing is loaded from another file or host.
	if m := regexp.MustCompile(`(?i)(src|href)\s*=|@import|url\(|https?://`).FindString(got); m != "" {
		t.Errorf("html references an external resource (%q)", m)
	}
}

func TestRenderHTML_escapesHelpText(t *testing.T) {
	root := &models.Node{Name: "evil", FullPath: []string{"evil"},
		Description: `<script>alert(1)</script> & "quotes"`,
		Children:    []*models.Node{{Name: "<b>x</b>", FullPath: []string{"evil", "<b>x</b>"}}}}
	got := htmlOf(t, root, nil)
	if strings.Contains(got, "<script>alert(1)") || strings.Contains(got, "<b>x</b>") {
		t.Errorf("help text is not escaped:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("escaped description missing:\n%s", got)
	}
}

func TestRenderHTML_respectsDepthFilterAndCommandsOnly(t *testing.T) {
	shallow := htmlOf(t, sampleTree(), func(o *render.Options) { o.MaxDepth = 1 })
	if !strings.Contains(shallow, ">remote<") || strings.Contains(shallow, ">add<") {
		t.Errorf("--depth=1 should keep remote and drop add:\n%s", shallow)
	}
	filtered := htmlOf(t, sampleTree(), func(o *render.Options) { o.Filter = "add" })
	if !strings.Contains(filtered, ">add<") || strings.Contains(filtered, ">commit<") {
		t.Errorf("--filter=add should keep add and drop commit:\n%s", filtered)
	}
	excluded := htmlOf(t, sampleTree(), func(o *render.Options) { o.Exclude = "remote" })
	if strings.Contains(excluded, ">remote<") {
		t.Errorf("--exclude=remote should drop remote:\n%s", excluded)
	}
	bare := htmlOf(t, sampleTree(), func(o *render.Options) { o.CommandsOnly = true })
	if strings.Contains(bare, "--message") || strings.Contains(bare, "&lt;name&gt;") {
		t.Errorf("--commands-only should hide flags and positionals:\n%s", bare)
	}
}

func TestRenderHTML_marksStubsAndCountsCommands(t *testing.T) {
	root := sampleTree()
	root.Children = append(root.Children, &models.Node{Name: "later", FullPath: []string{"git", "later"}, Stub: true})
	got := htmlOf(t, root, nil)
	if !strings.Contains(got, "not discovered") {
		t.Errorf("stub node is not marked:\n%s", got)
	}
	if !strings.Contains(got, "6 commands") {
		t.Errorf("header should count 6 commands:\n%s", got)
	}
}
