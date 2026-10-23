---
type: content
uid: uid-owlbear
name: Owlbear
content_type: monster
system: example
campaign: "[[Stormdark]]"
---

# Owlbear

The pack declares `monster` with a `visibility: gm` default — but the
pack's default is applied when the pipeline *emits* the note, and this note
was emitted without a `visibility:`. So the note-level default applies:
a note with a campaign is `party`.
