package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const workspaceRoot = `C:\Users\HUAWEI\Desktop\arcee`

type toolExecutionResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func supportedLocalTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case
		"glob",
		"find",
		"grep",
		"read", "read_file",
		"ls", "list_dir",
		"write", "write_file",
		"edit",
		"multiedit", "multi_edit",
		"copy",
		"mkdir",
		"stat",
		"move", "rename",
		"delete", "remove",
		"bash", "powershell", "shell":
		return true
	default:
		return false
	}
}

func executeLocalToolCalls(toolCalls []openAIToolCall) ([]chatMessage, error) {
	results := make([]chatMessage, 0, len(toolCalls))
	for _, call := range toolCalls {
		result := executeLocalToolCall(call)
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		results = append(results, chatMessage{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    string(raw),
		})
	}
	return results, nil
}

func executeLocalToolCall(call openAIToolCall) toolExecutionResult {
	name := strings.TrimSpace(call.Function.Name)
	switch strings.ToLower(name) {
	case "glob":
		return runGlobTool(call)
	case "find":
		return runFindTool(call)
	case "grep":
		return runGrepTool(call)
	case "read", "read_file":
		return runReadTool(call)
	case "ls", "list_dir":
		return runLSTool(call)
	case "write", "write_file":
		return runWriteTool(call)
	case "edit":
		return runEditTool(call)
	case "multiedit", "multi_edit":
		return runMultiEditTool(call)
	case "copy":
		return runCopyTool(call)
	case "mkdir":
		return runMkdirTool(call)
	case "stat":
		return runStatTool(call)
	case "move", "rename":
		return runMoveTool(call)
	case "delete", "remove":
		return runDeleteTool(call)
	case "bash", "powershell", "shell":
		return runShellTool(call)
	default:
		return toolExecutionResult{
			Name:  name,
			OK:    false,
			Error: "unsupported local tool",
		}
	}
}

func runGlobTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		pattern = strings.TrimSpace(args.Path)
	}
	if pattern == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing pattern"}
	}

	matches, err := globWorkspace(pattern)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if len(matches) == 0 {
		return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "[]"}
	}
	raw, _ := json.Marshal(matches)
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: string(raw)}
}

func runGrepTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing pattern"}
	}

	root := strings.TrimSpace(args.Path)
	if root == "" {
		root = "."
	}
	root, err := resolveWorkspacePath(root)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	matches := make([]string, 0, 32)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, args.Pattern) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", path, i+1, strings.TrimRight(line, "\r")))
				if len(matches) >= 200 {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: strings.Join(matches, "\n")}
}

func runFindTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing pattern"}
	}

	root := strings.TrimSpace(args.Path)
	if root == "" {
		root = "."
	}
	root, err := resolveWorkspacePath(root)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	matches := make([]string, 0, 64)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.Contains(strings.ToLower(d.Name()), strings.ToLower(args.Pattern)) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	sort.Strings(matches)
	raw, _ := json.Marshal(matches)
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: string(raw)}
}

func runReadTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path string `json:"path"`
		File string `json:"file"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	path := strings.TrimSpace(args.Path)
	if path == "" {
		path = strings.TrimSpace(args.File)
	}
	if path == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}
	path, err := resolveWorkspacePath(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	text := string(data)
	if len(text) > 12000 {
		text = text[:12000]
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: text}
}

func runLSTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	path := strings.TrimSpace(args.Path)
	if path == "" {
		path = "."
	}
	path, err := resolveWorkspacePath(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		label := entry.Name()
		if entry.IsDir() {
			label += "/"
		}
		lines = append(lines, label)
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: strings.Join(lines, "\n")}
}

func runWriteTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "written " + path}
}

func runEditTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}
	if args.OldString == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing old_string"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	text := string(data)
	if !strings.Contains(text, args.OldString) {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "old_string not found"}
	}

	var next string
	if args.ReplaceAll {
		next = strings.ReplaceAll(text, args.OldString, args.NewString)
	} else {
		next = strings.Replace(text, args.OldString, args.NewString, 1)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "edited " + path}
}

func runMultiEditTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path  string `json:"path"`
		Edits []struct {
			OldString  string `json:"old_string"`
			NewString  string `json:"new_string"`
			ReplaceAll bool   `json:"replace_all"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}
	if len(args.Edits) == 0 {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing edits"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	text := string(data)

	for _, edit := range args.Edits {
		if edit.OldString == "" {
			return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing old_string in edits"}
		}
		if !strings.Contains(text, edit.OldString) {
			return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "old_string not found: " + edit.OldString}
		}
		if edit.ReplaceAll {
			text = strings.ReplaceAll(text, edit.OldString, edit.NewString)
		} else {
			text = strings.Replace(text, edit.OldString, edit.NewString, 1)
		}
	}

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "edited " + path}
}

func runCopyTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		From string `json:"from"`
		To   string `json:"to"`
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	from := strings.TrimSpace(args.From)
	if from == "" {
		from = strings.TrimSpace(args.Path)
	}
	to := strings.TrimSpace(args.To)
	if to == "" {
		to = strings.TrimSpace(args.Dest)
	}
	if from == "" || to == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing from/to"}
	}

	fromPath, err := resolveWorkspacePath(from)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	toPath, err := resolveWorkspacePath(to)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	info, err := os.Stat(fromPath)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if info.IsDir() {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "copy only supports files"}
	}
	data, err := os.ReadFile(fromPath)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.WriteFile(toPath, data, 0o644); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: fmt.Sprintf("copied %s -> %s", fromPath, toPath)}
}

func runShellTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Command string `json:"command"`
		Cmd     string `json:"cmd"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	command := strings.TrimSpace(args.Command)
	if command == "" {
		command = strings.TrimSpace(args.Cmd)
	}
	if command == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing command"}
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", command)
	cmd.Dir = workspaceRoot
	output, err := cmd.CombinedOutput()
	text := string(output)
	if len(text) > 12000 {
		text = text[:12000]
	}
	if err != nil {
		return toolExecutionResult{
			Name:   call.Function.Name,
			OK:     false,
			Output: text,
			Error:  err.Error(),
		}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: text}
}

func runMkdirTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "created " + path}
}

func runStatTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	info, err := os.Stat(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	payload := map[string]any{
		"path":         path,
		"name":         info.Name(),
		"size":         info.Size(),
		"is_dir":       info.IsDir(),
		"mode":         info.Mode().String(),
		"modified_at":  info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
	}
	raw, _ := json.Marshal(payload)
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: string(raw)}
}

func runMoveTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		From string `json:"from"`
		To   string `json:"to"`
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	from := strings.TrimSpace(args.From)
	if from == "" {
		from = strings.TrimSpace(args.Path)
	}
	to := strings.TrimSpace(args.To)
	if to == "" {
		to = strings.TrimSpace(args.Dest)
	}
	if from == "" || to == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing from/to"}
	}

	fromPath, err := resolveWorkspacePath(from)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	toPath, err := resolveWorkspacePath(to)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if err := os.Rename(fromPath, toPath); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: fmt.Sprintf("moved %s -> %s", fromPath, toPath)}
}

func runDeleteTool(call openAIToolCall) toolExecutionResult {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	if strings.TrimSpace(args.Path) == "" {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "missing path"}
	}

	path, err := resolveWorkspacePath(args.Path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}
	info, err := os.Stat(path)
	if err != nil {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
	}

	if info.IsDir() && !args.Recursive {
		return toolExecutionResult{Name: call.Function.Name, OK: false, Error: "directory delete requires recursive=true"}
	}

	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
		}
	} else {
		if err := os.Remove(path); err != nil {
			return toolExecutionResult{Name: call.Function.Name, OK: false, Error: err.Error()}
		}
	}
	return toolExecutionResult{Name: call.Function.Name, OK: true, Output: "deleted " + path}
}

func appendToolMessages(messages []chatMessage, toolCalls []openAIToolCall, toolMessages []chatMessage) []chatMessage {
	next := make([]chatMessage, 0, len(messages)+1+len(toolMessages))
	next = append(next, messages...)
	next = append(next, chatMessage{
		Role: "assistant",
		ToolCalls: toolCalls,
	})
	next = append(next, toolMessages...)
	return next
}

func allSupportedLocalTools(toolCalls []openAIToolCall) bool {
	if len(toolCalls) == 0 {
		return false
	}
	for _, call := range toolCalls {
		if !supportedLocalTool(call.Function.Name) {
			return false
		}
	}
	return true
}

func resolveWorkspacePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, path)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(strings.ToLower(resolved), strings.ToLower(root)) {
		return "", fmt.Errorf("path outside workspace is not allowed")
	}
	return resolved, nil
}

func globWorkspace(pattern string) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	normalized := filepath.ToSlash(pattern)
	if !strings.Contains(normalized, "**") {
		target := pattern
		if !filepath.IsAbs(target) {
			target = filepath.Join(workspaceRoot, target)
		}
		matches, err := filepath.Glob(target)
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		return filterWorkspaceMatches(matches), nil
	}

	rootPart := normalized[:strings.Index(normalized, "**")]
	rootPart = strings.TrimSuffix(rootPart, "/")
	if rootPart == "" {
		rootPart = "."
	}
	rootPath, err := resolveWorkspacePath(filepath.FromSlash(rootPart))
	if err != nil {
		return nil, err
	}

	matches := make([]string, 0, 32)
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		candidate := filepath.ToSlash(path)
		ok, matchErr := pathMatch(filepath.ToSlash(filepath.Join(workspaceRoot, normalized)), candidate)
		if matchErr != nil {
			return matchErr
		}
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return filterWorkspaceMatches(matches), nil
}

func filterWorkspaceMatches(matches []string) []string {
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if resolved, err := resolveWorkspacePath(match); err == nil {
			out = append(out, resolved)
		}
	}
	return out
}

func pathMatch(pattern, candidate string) (bool, error) {
	patternParts := strings.Split(filepath.ToSlash(pattern), "/")
	candidateParts := strings.Split(filepath.ToSlash(candidate), "/")
	return matchParts(patternParts, candidateParts)
}

func matchParts(patternParts, candidateParts []string) (bool, error) {
	if len(patternParts) == 0 {
		return len(candidateParts) == 0, nil
	}
	if patternParts[0] == "**" {
		if len(patternParts) == 1 {
			return true, nil
		}
		for i := 0; i <= len(candidateParts); i++ {
			ok, err := matchParts(patternParts[1:], candidateParts[i:])
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	if len(candidateParts) == 0 {
		return false, nil
	}
	ok, err := filepath.Match(patternParts[0], candidateParts[0])
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return matchParts(patternParts[1:], candidateParts[1:])
}
