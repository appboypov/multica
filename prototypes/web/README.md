# Steer all work: live prototype

Clickable MagicPath prototype of `designs/screens/steer-all-work.pen`: https://designs.magicpath.ai/v1/calm-water-3032

`src/` is the submitted MagicPath component source; the PNGs are the five flows on the hosted build.

`created-issues/` shows an issue of every type made from its template on the hosted build: its page (`page-<type>.png`, `page-from-lane.png`), the squad board and the issue's preview there (`board.png`, `board-peek.png`, `peek-open-page.png`), and the worklane timeline with the issues in their release (`timeline.png`, `timeline-fresh.png`). The New issue dialog's "Fill example" fills a template with a worked example; it exists only in the prototype.

An issue's description is one Markdown field. The templates are the Markdown issue templates in `~/Work/the-archive` (`issue-management/templates/` for epic, story, bug, enhancement, refactor and chore; `linear/templates/` for spike and journey), without the sections that hold implementation details. The New issue fields only fill the chosen template's Markdown once; the issue keeps no separate fields. `markdown-description/` shows the dialog's Fields and Markdown tabs, the issue page's Preview and Markdown tabs, and an edit made in the Markdown.
