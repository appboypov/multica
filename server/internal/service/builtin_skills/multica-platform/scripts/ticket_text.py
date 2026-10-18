"""The text formats `tickets.py` reads and writes: YAML scalars, the `x-build:` map, frontmatter and Markdown."""

import json
import re

QUOTED_COMMENT_RE = re.compile(r"""^("(?:[^"\\]|\\.)*"|'(?:[^']|'')*')\s+#.*$""")
TOP_KEY_RE = re.compile(r"^([A-Za-z_][\w-]*)\s*:(.*)$")
FRONTMATTER_RE = re.compile(r"\A---\n(.*?)\n?---(?:\n|\Z)", re.S)
FENCE_RE = re.compile(r"^```[\w-]*\n(.*?)^```", re.M | re.S)


class TicketError(Exception):
    pass


def label_names(issue: dict) -> list[str]:
    return [str(label.get("name") if isinstance(label, dict) else label) for label in issue.get("labels") or []]


def one_label(names: list[str], prefix: str, where: str) -> str:
    """The value of the one label that starts with `prefix`; "" without one."""
    values = [n[len(prefix) :] for n in names if n.startswith(prefix)]
    if len(values) > 1:
        raise TicketError(f"{where}: carries more than one {prefix} label: {', '.join(values)}")
    return values[0] if values else ""


def keys_of(value) -> list[str]:
    """The issue keys of a comma-separated metadata value, in their order, each once."""
    return list(dict.fromkeys(k.strip() for k in str(value or "").split(",") if k.strip()))


# ---------------------------------------------------------------- scalars


def _scalar(value: str):
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        return [] if not inner else [_scalar(v.strip()) for v in inner.split(",")]
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        if value[0] == '"':
            try:
                return str(json.loads(value))
            except ValueError:
                pass
        return value[1:-1] if value[0] == '"' else value[1:-1].replace("''", "'")
    if value in ("true", "false"):
        return value == "true"
    if value in ("null", "~"):
        return ""
    if re.fullmatch(r"-?\d+", value):
        return int(value)
    return value


def yaml_value(value: str):
    value = value.strip()
    quoted = QUOTED_COMMENT_RE.match(value)
    if quoted:
        return _scalar(quoted.group(1))
    if not (len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'"):
        value = value.split(" #")[0].strip()
    return _scalar(value)


def quoted(value: str) -> str:
    """A double-quoted YAML scalar that `read_build` reads back exactly."""
    # str.splitlines splits on these characters, so they go in as escapes.
    text = json.dumps(value, ensure_ascii=False)
    return text.translate({c: f"\\u{c:04x}" for c in (0x85, 0x2028, 0x2029)})


def listed(value) -> list:
    """A property value as a list: a list as it is, any other value but empty in a list of one."""
    if isinstance(value, list):
        return value
    return [] if value in (None, "") else [value]


def _flag(value) -> bool:
    return str(value).lower() == "true"


def _one_of(value, allowed: tuple, where: str, field: str):
    """`value` when it is absent (`""`) or one of `allowed`; else the error naming them."""
    if value not in ("", *allowed):
        raise TicketError(f"{where}: {field} {value!r} is not one of {', '.join(allowed)}")
    return value


# ---------------------------------------------------------------- build map and frontmatter


def build_span(lines: list[str]):
    """(header index, end index, key indent) of the top-level `x-build:` map; None without one."""
    for i, line in enumerate(lines):
        m = TOP_KEY_RE.match(line)
        if not m or m.group(1) != "x-build":
            continue
        rest = m.group(2).split(" #")[0].strip()
        if rest not in ("", "{}"):
            raise TicketError("the x-build: map in .openspec.yaml must be a block map")
        end = i + 1
        while end < len(lines) and (not lines[end].strip() or lines[end][:1] in (" ", "\t")):
            end += 1
        while end > i + 1 and not lines[end - 1].strip():
            end -= 1
        body = [b for b in lines[i + 1 : end] if b.strip()]
        indent = min(len(b) - len(b.lstrip()) for b in body) if body else 2
        return i, end, indent
    return None


def read_keys(body: list[str], indent: int) -> dict:
    """The scalar keys at `indent`; nested values and list items are skipped."""
    data = {}
    for line in body:
        stripped = line.strip()
        if len(line) - len(line.lstrip()) != indent or stripped.startswith(("#", "- ")):
            continue
        key, sep, value = stripped.partition(":")
        if sep:
            data[key.strip()] = yaml_value(value)
    return data


def set_keys(body: list[str], indent: int, updates: dict) -> list[str]:
    """`body` with each key of `updates` set at `indent`, or removed when its value is None."""
    body = list(body)
    for key, value in updates.items():
        at = next(
            (
                j
                for j, b in enumerate(body)
                if len(b) - len(b.lstrip()) == indent and re.match(rf"{re.escape(key)}\s*:", b.strip())
            ),
            None,
        )
        new = [] if value is None else [" " * indent + f"{key}: {value}"]
        if at is None:
            body += new
            continue
        stop = at + 1
        while stop < len(body) and body[stop].strip():
            deeper = len(body[stop]) - len(body[stop].lstrip()) > indent
            if not (deeper or body[stop].lstrip().startswith("- ")):
                break
            stop += 1
        body[at:stop] = new
    return body


def read_build(text: str) -> dict:
    """The scalar keys of the `x-build:` map."""
    lines = text.splitlines()
    span = build_span(lines)
    if span is None:
        return {}
    i, end, indent = span
    return read_keys(lines[i + 1 : end], indent)


def write_build(text: str, updates: dict) -> str:
    """`text` with each key of `updates` set in the `x-build:` map, or removed when its value is None."""
    lines = text.splitlines()
    span = build_span(lines)
    if span is None:
        while lines and not lines[-1].strip():
            lines.pop()
        lines.append("x-build:")
        i, end, indent = len(lines) - 1, len(lines), 2
    else:
        i, end, indent = span
        if TOP_KEY_RE.match(lines[i]).group(2).split(" #")[0].strip() == "{}":
            lines[i] = "x-build:"
    lines[i + 1 : end] = set_keys(lines[i + 1 : end], indent, updates)
    return "\n".join(lines) + "\n"


def read_frontmatter(text: str) -> dict:
    m = FRONTMATTER_RE.match(text)
    return read_keys(m.group(1).splitlines(), 0) if m else {}

# ---------------------------------------------------------------- bodies


def headings(markdown: str) -> list[tuple[str, str | None]]:
    """Each line of a body with its `## ` heading text, None for a line that is no heading or lies in a ``` fence."""
    out, fence = [], ""
    for line in markdown.split("\n"):
        run = re.match(r"`{3,}", line)
        if fence:
            if run and len(run.group()) >= len(fence) and not line.strip().strip("`"):
                fence = ""
            out.append((line, None))
        elif run:
            fence = run.group()
            out.append((line, None))
        else:
            out.append((line, line[3:].strip() if line.startswith("## ") else None))
    return out


def sections(markdown: str) -> dict[str, list[str]]:
    """The lines under each `## ` heading of a body, by the heading in lowercase."""
    out, current = {}, None
    for line, heading in headings(markdown):
        if heading is not None:
            current = out.setdefault(heading.lower(), [])
        elif current is not None:
            current.append(line)
    return out


def text_of(lines: list[str]) -> str:
    return "\n".join(lines).strip()


def items_of(lines: list[str]) -> list[str]:
    return [line[2:].strip() for line in lines if line.startswith("- ")]


def with_section(markdown: str, heading: str, lines: list[str]) -> str:
    """`markdown` with its `## <heading>` section holding `lines`, added at the end when absent."""
    out, skipping, found = [], False, False
    for line, title in headings(markdown):
        if title is not None:
            skipping = title.lower() == heading.lower()
            if skipping:
                out += [f"## {heading}", "", *lines, ""]
                found = True
                continue
        if not skipping:
            out.append(line)
    if not found:
        out += ["", f"## {heading}", "", *lines]
    return "\n".join(out).strip("\n") + "\n"


def spike_build(markdown: str) -> dict:
    """The keys of the `yaml` block under `## Build`; {} without one."""
    m = FENCE_RE.search(text_of(sections(markdown).get("build", [])) + "\n")
    return read_keys(m.group(1).splitlines(), 0) if m else {}


def write_spike_build(markdown: str, updates: dict) -> str:
    """`markdown` with each key of `updates` set in the `## Build` block, or removed when its value is None."""
    m = FENCE_RE.search(text_of(sections(markdown).get("build", [])) + "\n")
    lines = set_keys(m.group(1).splitlines() if m else [], 0, updates)
    return with_section(markdown, "Build", ["```yaml", *lines, "```"])
