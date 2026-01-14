---
trigger:
  - ui/**
---
# Interface Directives (UI/UX Rules)

**1. Semantic (Concepts over Schema)**
- Build interfaces around *Concepts* (e.g., "Customer journey"), not database tables.
- Use semantic grammar in CLIs (verbs/nouns) rather than obscure flags.
- **Agent Action:** Structure data to match human workflows (Draft -> Review), not storage logic.

**2. Reactive (Tangibility)**
- **Web:** Use Optimistic UI. Vanish archived items immediately; don't wait for server round-trips.
- **CLI:** Prioritize interactive selection (fzf, menus) over asking for typed IDs.

**3. Transparent (Observability)**
- **Web:** Never obscure background work. Show global indicators for syncs/exports.
- **CLI:** "Silence is failure." Output diffs for state changes.
- **Agent Action:** Always generate a **Dry Run** preview (Before/After) before complex tasks.

**4. Consistent (Invariance)**
- Adhere to a rigid Design System. "Success" color is never neutral.
- Maintain **Ontological Consistency**. Don't switch vocabulary (e.g., "Workspace" -> "Folder").

**5. Forgiving (User Sovereignty)**
- **Undo > Confirm:** Replace "Are you sure?" dialogs with Undo capabilities.
- **Staged Execution:** Draft -> Review -> Commit.
- **Agent Action:** Explicit confirmation is ONLY for destructive actions (delete/overwrite).

**6. High-Density (Signal-to-Noise)**
- **Web:** White space is for grouping, not decoration. High data density is good if hierarchy is clear.
- **CLI:** Use columns, headers, and semantic coloring for scannability.
- **Agent Output:** Answer first, context second.

**7. Feedback (System Vitality)**
- Acknowledge every interaction instantly (micro-animations, ripples).
- **CLI:** Spinners for >200ms ops.
- **Agent Behavior:** Emit status tokens during deep reasoning.
