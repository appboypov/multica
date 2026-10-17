#!/bin/sh
# Links Multica's built-in skill multica-platform into every local harness, so a session
# outside a Multica run reads the same skill, references and scripts a run gets.
#   ~/Work/skills          read by every harness through ~/.agents/skills, ~/.claude/skills and the rest
#   ~/.codex/skills, ~/.gemini/skills, ~/.kiro/skills, ~/.qwen/skills,
#   ~/.config/goose/skills, ~/.copilot/skills   harnesses with their own skills folder

set -eu

repo=$(CDPATH= cd "$(dirname "$0")/.." && pwd -P)
skill="$repo/server/internal/service/builtin_skills/multica-platform"

for dir in Work/skills .codex/skills .gemini/skills .kiro/skills .qwen/skills .config/goose/skills .copilot/skills; do
    mkdir -p "$HOME/$dir"
    link="$HOME/$dir/multica-platform"
    [ -L "$link" ] && [ "$(readlink "$link")" = "$skill" ] && continue
    if [ -L "$link" ] && [ ! -e "$link" ]; then
        rm "$link"
    elif [ -e "$link" ] || [ -L "$link" ]; then
        echo "skipped $link: something else already stands there" >&2
        continue
    fi
    ln -s "$skill" "$link"
    echo "linked $link"
done
