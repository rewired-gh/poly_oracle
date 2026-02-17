#!/bin/bash

# Read the JSON input from Claude Code
input=$(cat)

# Extract the file path using jq
# Note: tool_input.file_path is standard for Edit/Write tools
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only proceed if the file exists and is a .go file
if [[ "$file_path" == *.go ]] && [ -f "$file_path" ]; then
    echo "--- Checking Go Quality: $file_path ---"
    
    # 1. Format the specific file
    gofmt -s -w "$file_path"
    
    # 2. Run golangci-lint on the specific file
    # We use --fix to let the linter resolve what it can automatically
    lint_output=$(golangci-lint run --fix "$(dirname "$file_path")" 2>&1)
    
    if [ $? -eq 0 ]; then
        echo "✅ $file_path passed checks."
        exit 0
    else
        echo "❌ Linter issues in $file_path:" >&2
        echo "$lint_output" >&2
        # Exit 2 sends the stderr back to Claude as a "block" signal
        # This forces the agent to read the errors and fix them.
        exit 2
    fi
fi

exit 0