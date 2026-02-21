# claude-deck — Product & Technical Specification

## 1. What This Is

A terminal application that multiplexes Claude Code sessions using a card-based queue instead of spatial panes. The user runs multiple Claude Code conversations simultaneously. Instead of splitting the terminal into side-by-side panes (which doesn't scale past 2-3 sessions), sessions are represented as cards. Cards that need user input accumulate in a FIFO queue. The user works through the queue one card at a time.

The app has two UI modes: **Normal Mode** (full overview of all sessions) and **Focus Mode** (compressed, distraction-free queue-drain interface).

---

## 2. Core Concepts

### 2.1 Session States

Every session is in exactly one of two states:

| State | Meaning | Visual |
|-------|---------|--------|
| **Working** | Claude Code subprocess is running. User cannot interact. | Blue spinner `⟳` |
| **Needs You** | Claude finished. User's turn to respond or dismiss. | Amber dot `●` |

There is no separate "idle" or "completed" state. When Claude finishes — whether it asked a question, gave options, or just said "done" — the session enters **Needs You**. The user decides what to do: reply, or dismiss/close the session.

When a session is Working but waiting for a subprocess slot (all `max_concurrent` slots occupied), the spinner shows a muted style: `⟳` in dim instead of blue. This is a visual-only distinction; the state is still Working. The transition from slot-waiting to running is communicated via a `SlotAcquiredMsg` sent from the goroutine after the semaphore is acquired (see Section 8 Message Types). The session has a `SlotAcquired` boolean field that the UI reads to choose the spinner style.

### 2.2 The Queue

The queue is a FIFO list of all sessions in the **Needs You** state, ordered by the time they entered that state (oldest first). The queue is the central navigation mechanism. Arrow keys (←→) move through the queue only — they skip Working sessions entirely.

The queue rebuilds automatically whenever a session changes state.

### 2.3 The Prompt-Advance Loop

The core interaction loop:

1. User sees the current queue card (Claude's last output)
2. User types a reply and presses Enter
3. The session transitions to **Working**
4. The view auto-advances to the next card in the queue
5. If the queue is empty:
   - **Focus Mode:** Show a centered waiting screen (`⟳ {N} sessions working...`) until a session finishes.
   - **Normal Mode:** Show the last active session's chat view (read-only) with input disabled. "Last active" means the session the user most recently sent a message to. If no session has been interacted with yet (e.g. all sessions were just created from CLI args), default to the first session by creation order. The user can still toggle the card strip, view Working sessions via `Ctrl+1–9`, or create new sessions.

This loop is the same in both Normal and Focus modes, but Focus Mode compresses the UI to minimize distractions.

---

## 3. UI Modes

### 3.1 Normal Mode

Full overview. The screen is divided into these regions from top to bottom:

```
┌──────────────────────────────────────────────────────────────┐
│ TOP BAR: brand, session count, waiting badge                 │
├──────────────────────────────────────────────────────────────┤
│ CARD STRIP (hidden by default, toggle with V)                │
├──────────────────────────────────────────────────────────────┤
│ QUEUE NAV: ← prev  ━━ ── ── ──  next →   2/4               │
├──────────────────────────────────────────────────────────────┤
│ SESSION HEADER: name, state badge, wait time                 │
│                                                              │
│ CHAT VIEW: scrollable conversation history                   │
│   you › refactor the auth middleware                         │
│   claude › I've refactored it. Should I also update...       │
│                                                              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ INPUT BAR: > type here...                                    │
│ HINT: 3 more waiting · press → after sending                 │
├──────────────────────────────────────────────────────────────┤
│ HELP BAR: ←→ nav · Ctrl+U/D scroll · Enter send · F focus · V strip │
└──────────────────────────────────────────────────────────────┘
```

#### Top Bar
- Left: `◆ claude-deck` brand + session count (e.g. "6 sessions")
- Right: amber badge showing queue count when > 0 (e.g. `● 3 waiting`). Clicking/selecting this jumps to the first queue item.

#### Card Strip
- **Hidden by default.** Toggle visibility with `V`. This reclaims ~3 lines of vertical space for the chat view, which matters on small displays.
- When visible: horizontal row of cards, one per session.
- Each card shows: session name (truncated to ~14 chars), state badge (●/⟳), preview of last output (truncated to ~20 chars, single line), wait time if in Needs You state.
- Active card has an amber border. Working cards have a blue border (or dim border if queued for a subprocess slot). Others have a dim border.
- Cards that are in Needs You but not active show an amber notification dot in the top-right corner.
- If there are more cards than fit the terminal width, show `+N` overflow indicator on the right.

#### Queue Navigation Bar
- Only visible when queue has items.
- Shows: `← prev` button, dot indicators (━━ for active, ── for others), `next →` button, position counter (e.g. `2/4`).
- ←→ arrow keys cycle through the queue, wrapping at edges.

#### Session Header
- Separator line (full-width thin rule).
- Session name (bold), state badge, wait time (e.g. "waiting 2m").

#### Chat View
- Scrollable view of the full conversation history for the active session.
- Each message shows role prefix (`you ›` in blue, `claude ›` in amber) followed by the message text, word-wrapped to terminal width minus padding. User messages have a subtle `UserBg` background stripe across the full line; the `claude ›` role prefix has a `ClaudeBg` background highlight.
- Messages separated by a blank line.
- If session is Working, show a pulsing indicator at the bottom: `claude › thinking... (12s)`. If a status hint is available from stream parsing (e.g. tool use), show it: `claude › editing auth.ts... (12s)`. If the session is waiting for a subprocess slot, show: `claude › waiting for slot... (5s)` in muted text.
- The chat view fills all remaining vertical space between the session header and input bar.
- Auto-scrolls to bottom on new messages and when navigating to a different session. Scroll position is not preserved per session — switching sessions always resets to the bottom. This is acceptable for MVP; per-session viewport state is a future consideration.
- Scroll with Ctrl+U/Ctrl+D (half page) or Ctrl+↑/Ctrl+↓ (line). Mouse scroll wheel also supported if terminal allows.
- When viewing a Working session (reached via Ctrl+1–Ctrl+9 shortcuts), the chat view is read-only and shows a pulsing indicator at the bottom.
- Claude's output is rendered as plain text. No markdown parsing or syntax highlighting in MVP. Code blocks appear as-is with their backtick fences.

#### Input Bar
- Separator line.
- Text input with prompt `> `. Placeholder text shows `Reply to {name}...` when in Needs You, or `Claude is working...` when Working (input disabled).
- Shift+Enter inserts a newline. The input bar expands up to 5 lines, then scrolls internally.
- Below the input: hint text showing remaining queue count and navigation reminder.

#### Help Bar
- Single line at bottom showing key bindings: `←→ nav · Ctrl+U/D scroll · Enter send · F focus · V strip · Ctrl+N new`

### 3.2 Focus Mode

Compressed, distraction-free queue-drain interface. Activated by pressing `F` in Normal Mode. The goal is to minimize context-switching overhead — you see only what the current session needs and respond, then move on.

```
┌──────────────────────────────────────────────────────────────┐
│ ◆ focus                                            2 working │
│ ━━━━━━━━━━━━━━━━━━━░░░░░░░░░ 5/8               +2 incoming  │
├──────────────────────────────────────────────────────────────┤
│ auth-refactor                                                │
│                                                              │
│ I've refactored the auth middleware to use JWT. The tests    │
│ pass but I noticed the refresh token logic might need        │
│ updating. Should I proceed with updating the refresh flow,   │
│ or do you want to handle that separately?                    │
│                                                              │
│                                                              │
│                                                              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ > _                                                          │
│                                                              │
│ Enter send · S skip · Ctrl+W dismiss · Ctrl+Enter default reply · Esc exit │
└──────────────────────────────────────────────────────────────┘
```

#### What Changes vs Normal Mode

| Element | Normal Mode | Focus Mode |
|---------|-------------|------------|
| Card strip | Toggleable (hidden by default) | **Hidden** |
| Queue nav dots | Visible | **Replaced by progress bar** |
| Chat history | Full scrollable history | **Only Claude's last message** |
| Session header | Name + badge + time | **Name only, small/muted** |
| Input behavior on empty Enter | Does nothing | **Does nothing** (same as Normal) |
| Exit method | Any navigation | **Esc** |
| Help bar | Full keybindings | **Only send + skip + dismiss + default reply + scroll + exit** |

#### Focus Header
- Left: `◆ focus` — simple mode indicator.
- Right: `{N} working` — how many sessions are still processing.

#### Progress Bar
- Visual bar showing cleared vs remaining: `━━━━━━━━━░░░░ 5/8`
- Filled segments (━) = cleared count. Empty segments (░) = remaining in queue.
- Total starts as the queue length when Focus Mode is entered. Cleared segments increment as you respond. If working sessions finish and join the queue while in Focus Mode, the denominator **extends** to include them (e.g. `5/8` becomes `5/10`). New segments appear as empty (░) at the end of the bar. This avoids the jarring experience of a full bar resetting to empty. The `+N incoming` label to the right of the bar shows how many new items arrived since focus entry, giving the user context for why the bar grew.

#### Compressed Card View
- Session name in muted text at top.
- Claude's last output only — no history, no user messages, no role prefixes. Just the text you need to respond to. Word-wrapped to terminal width minus padding.
- This text fills the available vertical space.

#### Focus Input Bar
- Amber separator line (not dim like normal mode).
- Same `> ` prompt.
- Empty Enter does nothing — same as Normal Mode. The user must type a response or skip the card.
- **Quick reply**: The user can configure a `default_reply` in config. In Focus Mode, pressing `Ctrl+Enter` sends the default reply (e.g. "lgtm, continue") and advances. This is an explicit action, not an accidental Enter press.
- Same multi-line input behavior as Normal Mode (Shift+Enter for newlines, up to 5 lines).
- **Dismiss**: Pressing `Ctrl+W` dismisses the current session (removes it from the queue and session list), decrements `FocusTotal` by 1 (so the progress bar stays accurate), and advances to the next card. This lets the user close "done" sessions without leaving Focus Mode. (Only Needs You sessions appear in Focus Mode, so no subprocess cancellation applies here — see Normal Mode Ctrl+W in Section 6 for Working session dismissal.)

#### Focus Exit
- Pressing `Esc` exits Focus Mode immediately and returns to Normal Mode.
- On exit, show notification: `Exited focus mode`.

#### Skip Card
- Pressing `S` **when the input is empty** moves the current card to the **end of the queue** and advances to the next card. This lets the user defer cards that need more thought without leaving Focus Mode. When the input contains text, `S` types the character normally.
- If the current card is the only one in the queue, `S` does nothing (nowhere to skip to).

#### Focus Auto-Advance
- After sending a reply, instantly advance to next queue card. No transition, no delay.
- If queue is empty, show a centered waiting screen: `⟳ {N} sessions working... waiting for responses`.
- When a new card enters the queue while on the waiting screen, immediately show it.

#### Focus Auto-Suggest
- When 3 or more cards are queued in Normal Mode, show a transient notification: `{N} cards waiting — press F to focus`.
- This notification only shows once per queue buildup (reset when queue drops to 0, so it does not re-fire on every oscillation around the threshold).

---

## 4. Notifications

Transient notifications appear below the queue nav (Normal Mode) or below the progress bar (Focus Mode). They auto-dismiss after 3 seconds.

Events that trigger notifications:
- A session enters Needs You state: `+ {session_name} needs you`
- Focus auto-suggest: `{N} cards waiting — press F to focus`
- Focus exit: `Exited focus mode`
- Error in a session: `✗ {session_name}: {error_summary}`

Maximum 5 visible notification slots. Slots 1–4 show the 4 most recent notifications. Slot 5 is the overflow indicator: when there are more than 4 active notifications, slot 5 shows `…and N more` (where N is the count of notifications beyond the 4 shown). As notifications auto-dismiss (3s), the overflow count decrements. When the overflow count reaches 0, slot 5 disappears and older notifications promote into slots 1–4. Notifications stack vertically, each taking 1 line.

Terminal bell (`\a`) fires when a session enters the Needs You queue (configurable).

---

## 5. Key Bindings

### Global (both modes)
| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit (press twice within 1s if sessions are running) |
| `Shift+Enter` | Insert newline in input |

### Normal Mode
| Key | Action |
|-----|--------|
| `←` / `Shift+Tab` | Previous card in queue |
| `→` / `Tab` | Next card in queue |
| `Enter` | Send input to active session |
| `F` | Enter Focus Mode |
| `V` | Toggle card strip visibility |
| `Ctrl+N` | Create new session (opens name+prompt input) |
| `Ctrl+W` | Close/dismiss active session |
| `Ctrl+U` / `Ctrl+D` | Scroll chat up/down half page |
| `Ctrl+↑` / `Ctrl+↓` | Scroll chat up/down one line |
| `Alt+1` – `Alt+9` | Jump to queue position N. No-op if N exceeds the current queue length. |
| `Ctrl+1` – `Ctrl+9` | View session N (by current position in the Sessions slice, including Working sessions). After a session is dismissed, remaining sessions shift down — e.g. if session 1 is dismissed, what was session 2 becomes Ctrl+1. No-op if N exceeds the number of sessions. Input remains disabled for Working sessions. This is a temporary peek — pressing `←`/`→` returns to the queue (jumping to the current queue position), as arrow keys always navigate the queue. |

Note: `F`, `V`, Tab, and Shift+Tab only apply when the text input is empty. When the input contains text, these keys type normally (Tab inserts literal whitespace for multi-line editing, `F` and `V` type their respective characters). This is because the Bubbletea textarea component consumes single-character keypresses when focused — the shortcut handler checks `m.Input.Value() == ""` before dispatching these bindings.

Note: `Alt+1`–`Alt+9` and `Ctrl+1`–`Ctrl+9` may both be intercepted by some terminal emulators (e.g. iTerm2 uses `Ctrl+1`–`Ctrl+9` for tab switching, others use `Alt+N`). This is a known limitation. Users affected by this can rely on `←`/`→` queue navigation. If both modifier sets are intercepted, the user's only option is to reconfigure their terminal emulator or use arrow-key navigation.

### Focus Mode
| Key | Action |
|-----|--------|
| `Enter` (with text) | Send input, advance to next card |
| `Enter` (empty) | Does nothing |
| `Ctrl+Enter` | Send default reply, advance to next card |
| `S` (input empty) | Skip current card (move to end of queue) |
| `Ctrl+W` | Dismiss current session and advance |
| `Ctrl+U` / `Ctrl+D` | Scroll output up/down half page |
| `Esc` | Exit Focus Mode |

Note: `S` for skip only fires when the text input is empty. When the input contains text, `S` types normally. In Focus Mode, arrow keys and number keys are **not bound** to navigation. You cannot cherry-pick cards — you work the queue in order, but you can skip cards to the back with `S`.

### Mouse

Mouse scroll wheel scrolls the chat view in Normal Mode and the output view in Focus Mode. No other mouse interactions are supported in MVP — card strip clicks and queue dot clicks are reserved for future versions. Bubbletea mouse events are enabled.

---

## 6. Session Lifecycle

### Creating a Session

**From CLI arguments:**
```bash
claude-deck "name:prompt" "name2:prompt2" ...
```
Each argument is split on the **first** `:` only. Everything before the first colon is the session name; everything after is the prompt. For example, `"db:normalize: the users table"` → name `db`, prompt `normalize: the users table`. If an argument contains no colon, or the portion before the colon is empty (e.g. `":do something"`), it is treated as a prompt with an auto-generated name (`s1`, `s2`, etc.). If the portion after the colon is empty or whitespace-only (e.g. `"name:"` or `"name:  "`), the argument is skipped with a stderr warning (e.g. `Warning: empty prompt for "name", skipping`).

**From within the app (Ctrl+N):**
1. An overlay/input appears asking for session name.
2. After entering name and pressing Enter, a second input asks for the initial prompt. Pressing Backspace on an empty prompt input returns to the name step.
3. Session is created in Working state, prompt is dispatched.
4. Pressing `Esc` at any step cancels the overlay entirely and returns to the previous view.

### Claude Code Integration

Each session maps to a Claude Code subprocess call:

**First message:**
```bash
claude --print --output-format stream-json --session-id deck-{runid}-{name}-{shortid} "{prompt}"
```

**Follow-up messages (only after a previous successful completion):**
```bash
claude --print --output-format stream-json --resume --session-id deck-{runid}-{name}-{shortid} "{prompt}"
```

Key flags:
- `--print`: Non-interactive mode. Single turn, exits after response.
- `--output-format stream-json`: Streaming JSON output. Each line is a JSON object with a `type` field. The app parses these incrementally to extract status updates (see Streaming Status below). The final assistant message text is assembled from `assistant` type events.
- `--session-id`: Persistent conversation ID. Allows `--resume` to continue the conversation. `runid` is a per-launch identifier (first 8 hex characters of a UUIDv4 generated once at app startup). `shortid` is the first 8 hex characters of a per-session UUIDv4. The `runid` prevents collisions with session IDs from previous runs of claude-deck — without it, `--resume` could accidentally resume a stale conversation from a prior run. Example: `deck-b7e2f1a0-auth-a3f1b2c9`.
- `--resume`: Continue an existing session instead of starting fresh. **Only added if the session has had at least one successful subprocess completion** (tracked via `CompletedOnce` flag on the Session struct). If the first subprocess is killed (timeout, Ctrl+W) before completing, Claude may not have persisted the session, so `--resume` would fail or start a blank conversation. By gating on `CompletedOnce`, retries after a failed first message correctly omit `--resume`.

The subprocess runs in a goroutine. When it completes, it sends a message to the Bubbletea event loop with the output text. The session transitions from Working to Needs You.

**Streaming status:** Using `--output-format stream-json`, the app reads subprocess stdout line by line. Each line is a JSON object. The app looks for events that indicate current activity (e.g. tool use events containing tool names). When detected, the spinner updates to show a status hint: `⟳ editing auth.ts... (12s)` instead of just `⟳ working for 12s...`. This gives the user meaningful feedback about what each session is doing without implementing full streaming output. The final response text is assembled from the stream events when the subprocess completes. If JSON parsing fails for any line, that line is silently skipped — the app degrades to timer-only feedback.

**Error handling:** If the subprocess exits with a non-zero code, the session still transitions to Needs You, but the output shows the error. The user can retry by sending another message, or close the session.

**Stderr handling:** Stderr is captured separately. On non-zero exit, stderr is prepended to the output shown to the user, formatted as `[stderr] {text}`. On zero exit, stderr is discarded (Claude CLI writes progress info to stderr that is not useful in `--print` mode).

### Closing a Session

`Ctrl+W` in Normal Mode dismisses the active session. If the session is in **Needs You** state, it is removed immediately from the session list and queue. If the session is **Working**, the first `Ctrl+W` shows a confirmation notification: `{name} is working — Ctrl+W again to dismiss`. A second `Ctrl+W` within 2 seconds cancels the subprocess context (killing the process), discards the result, and removes the session. This prevents accidental loss of in-flight work.

**Queue index adjustment after dismissal:** After removing a session, rebuild the queue and adjust `QueueIndex`. If the dismissed session was at the current queue position, advance to the next card at the same index (which is now the next card in the queue). If `QueueIndex` exceeds the new queue length, wrap to 0. If the queue is now empty, clear `ActiveID` (or show the empty-queue state). If `len(Sessions) == 0`, enter the empty state (see Section 11, Empty State). The view should always land on a valid queue card after dismissal — never on a stale index.

In Focus Mode, `Ctrl+W` also dismisses the current session and advances to the next card in the queue. (Only Needs You sessions appear in Focus Mode, so no confirmation is needed — see Section 3.2.)

### Overlay Keyboard Isolation

When the new-session overlay (Ctrl+N) is active, all other keybindings are suppressed. Arrow keys, `F`, `V`, `Ctrl+W`, number shortcuts, etc. do not fire. Only the overlay's own inputs (text entry, Enter to confirm, Backspace, Esc to cancel) are active. This prevents the user from accidentally navigating, dismissing sessions, or entering Focus Mode while creating a new session.

---

## 7. Configuration

Stored at the OS-standard config directory, resolved via `os.UserConfigDir()`:
- **Linux:** `~/.config/claude-deck/config.json`
- **macOS:** `~/Library/Application Support/claude-deck/config.json`
- **Windows:** `%APPDATA%\claude-deck\config.json` (e.g. `C:\Users\X\AppData\Roaming\claude-deck\config.json`)

Created with defaults on first run if not present.

```json
{
  "default_reply": "lgtm, continue",
  "focus_threshold": 3,
  "bell_on_queue": true,
  "max_concurrent": 5,
  "subprocess_timeout": "10m",
  "allowed_tools": []
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `default_reply` | string | `"lgtm, continue"` | What `Ctrl+Enter` sends in Focus Mode |
| `focus_threshold` | int | `3` | Queue size that triggers the focus mode nudge |
| `bell_on_queue` | bool | `true` | Play terminal bell when a session enters the queue |
| `max_concurrent` | int | `5` | Max concurrent claude subprocesses |
| `subprocess_timeout` | string | `"10m"` | Timeout per subprocess. Go duration string format only (e.g. `"30s"`, `"5m"`, `"10m"`). Parsed via `time.ParseDuration`. |
| `allowed_tools` | []string | `[]` (empty = no restriction) | If non-empty, passed as `--allowedTools` to each subprocess. See Section 9.6. |

Config is loaded once at startup. Changes to `config.json` while the app is running have no effect until restart.

---

## 8. Technical Architecture

### Stack

- **Language:** Go
- **TUI framework:** Bubbletea (Elm-architecture TUI framework)
- **Styling:** Lip Gloss (CSS-like terminal styling)
- **Subprocess:** `os/exec` calling `claude` CLI

### Project Structure

```
claude-deck/
├── main.go                         CLI entry, arg parsing, mock detection
├── go.mod
├── Makefile
└── internal/
    ├── session/
    │   ├── session.go              Session struct, state, Claude Code subprocess
    │   ├── proc_unix.go            Process group setup + kill (Unix/macOS) [build tag: !windows]
    │   ├── proc_windows.go         Job Object setup + kill (Windows) [build tag: windows]
    │   └── mock.go                 Mock responses for UI testing without claude CLI
    ├── queue/
    │   └── queue.go                FIFO queue, rebuild from session list
    ├── ui/
    │   ├── model.go                Bubbletea model, Init/Update/View, state machine
    │   ├── styles.go               Lip Gloss style definitions
    │   ├── view_normal.go          Normal mode rendering
    │   ├── view_focus.go            Focus mode rendering
    │   ├── input.go                Text input component (wraps bubbletea textarea, 5-line max)
    │   └── overlay.go              New session overlay (name + prompt input)
    └── config/
        └── config.go               Config loading/saving from os.UserConfigDir()/claude-deck/
```

### Bubbletea Model

The central `Model` struct:

```go
type Model struct {
    // Sessions
    Sessions    []*session.Session
    Queue       queue.Queue
    ActiveID    string              // ID of currently viewed session
    QueueIndex  int                 // Position in queue

    // UI Mode
    Mode        Mode                // NormalMode or FocusMode
    ShowStrip   bool                // Card strip visibility toggle

    // Input
    Input       textarea.Model      // Bubbletea textarea component (5-line max height)
    DefaultReply string

    // Focus state
    FocusCleared int                // Cards cleared in current focus session
    FocusTotal   int                // Queue length at Focus Mode entry (progress bar denominator, grows with incoming)
    FocusIncoming int               // Cards that arrived after focus entry (for +N incoming label)

    // Notifications
    Notifs      []Notification

    // Terminal
    Width       int
    Height      int

    // Overlay
    Overlay     *OverlayState       // Non-nil when new-session overlay is active

    // Dismiss confirmation
    DismissConfirmID  string        // Session ID awaiting second Ctrl+W
    DismissConfirmAt  time.Time     // When first Ctrl+W was pressed (expires after 2s)

    // Subprocess management
    RunID       string              // Per-launch ID (8 hex chars), used in session IDs to prevent cross-run collisions
    Sem         chan struct{}        // Buffered channel semaphore, size = config.MaxConcurrent, initialized after config load
    RootCtx     context.Context     // THE SAME context passed to tea.WithContext — cancelled by defer cancel() in main() after p.Run() returns
    RootCancel  context.CancelFunc  // Called by defer cancel() in main(). NOTE: Bubbletea does NOT cancel this context — the defer in main() does.
    ShutdownWg  sync.WaitGroup     // Tracks active subprocess goroutines; main() waits on this after cancel

    // Config
    Config      config.Config
    MockMode    bool
}
```

Each `Session` struct includes:
```go
type Session struct {
    ID            string
    Name          string
    State         SessionState       // Working or NeedsInput
    SlotAcquired  bool               // True once semaphore acquired (false = queued for slot)
    CompletedOnce bool               // True after first successful subprocess completion
    CancelFunc    context.CancelFunc // Cancels the current subprocess context. Set by handleSend(), called by dismiss handler. Nil when not Working.
    StatusHint    string             // Current activity hint from stream parsing (e.g. "editing auth.ts")
    History       []Message
    EnteredQueue  time.Time
    SkippedAt     time.Time          // Set/updated each time the card is skipped in Focus Mode (zero value = not skipped). Used for queue sort order only — EnteredQueue is preserved for "waiting Xm" display.
    StartedAt     time.Time          // When the current Send was initiated (for timeout + elapsed display)
    // ...
}
```

### Message Types

```go
// From subprocess goroutines
type SessionDoneMsg struct { Result session.SendResult }
type SlotAcquiredMsg struct { SessionID string }              // Semaphore acquired, subprocess launching
type StatusHintMsg struct { SessionID string; Hint string }   // Activity update from stream parsing

// Time-based updates (1/sec)
type TickMsg time.Time
```

### Update Flow

```
User presses Enter with text
  → handleSend()
    → active.History = append(active.History, Message{Role: "user", Text: prompt})
    → active.State = Working
    → active.SlotAcquired = false
    → active.StatusHint = ""
    → active.StartedAt = time.Now()
    → m.Input = ""
    → m.Queue.Rebuild(m.Sessions)
    → if FocusMode: m.FocusCleared++
    → advance to next queue item (if queue is empty, ActiveID stays on the session
      just replied to — now Working — and the view shows its read-only chat history)
    → return cmd: sendToSession(active, prompt)
      → goroutine: acquires semaphore → sends SlotAcquiredMsg
      → reads stream-json stdout line by line → sends StatusHintMsg
      → on complete: sends SessionDoneMsg

User presses Ctrl+Enter in Focus Mode
  → handleSend() with m.DefaultReply as prompt
  → same flow as Enter with text above

User presses S in Focus Mode (input must be empty)
  → move current card to end of queue
  → set current card's SkippedAt = time.Now() (updated on every skip, not just the first)
  → m.Queue.Rebuild(m.Sessions) (sorts by SkippedAt if set, else EnteredQueue)
  → advance to next queue item
  → Note: EnteredQueue is NOT modified — the "waiting Xm" timer preserves the original wait time

User presses Ctrl+W in Focus Mode
  → remove current session from session list and queue
  → m.FocusTotal-- (keep progress bar accurate — dismissed cards reduce the denominator)
  → m.Queue.Rebuild(m.Sessions)
  → advance to next queue item (if queue empty, show waiting screen or auto-exit)

User presses Ctrl+W on a Working session (Normal Mode)
  → if m.DismissConfirmID == active.ID && time.Since(m.DismissConfirmAt) < 2s:
    → active.CancelFunc() (cancels subprocess context, triggers process tree kill)
    → active.CancelFunc = nil
    → remove session from session list
    → m.Queue.Rebuild(m.Sessions)
    → clear confirmation state (m.DismissConfirmID = "", m.DismissConfirmAt = zero)
    → adjust QueueIndex / ActiveID (see "Queue index adjustment after dismissal" below)
  → else:
    → m.DismissConfirmID = active.ID
    → m.DismissConfirmAt = time.Now()
    → append notification: "{name} is working — Ctrl+W again to dismiss"

SlotAcquiredMsg received
  → find session by ID (if nil — session was dismissed — discard and break)
  → session.SlotAcquired = true (UI switches from dim to blue spinner)

StatusHintMsg received
  → find session by ID (if nil, discard and break)
  → session.StatusHint = msg.Hint

SessionDoneMsg received
  → find session by ID (if nil, discard and break)
  → session.State = NeedsInput (applied here on main goroutine, not in Send)
  → session.SlotAcquired = false
  → if msg.Result.Err == nil: session.CompletedOnce = true (only on success — see Section 6 --resume gating)
  → session.StatusHint = ""
  → session.EnteredQueue = msg.CompletedAt
  → session.History = append(session.History, claudeMessage)
  → sanitize output (strip ANSI, control chars)
  → m.Queue.Rebuild(m.Sessions)
  → if FocusMode: m.FocusTotal++ ; m.FocusIncoming++ (extend progress bar)
  → append notification
  → if no active session: focus this one
  → if bell_on_queue: write \a

TickMsg received (every 1s)
  → expire notifications older than 3s
  → expire dismiss confirmation if older than 2s
  → check focus auto-suggest threshold
```

### Session.Send()

```go
func (s *Session) Send(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program) SendResult {
    // ctx already carries the timeout (created BEFORE Send is called — see below)
    //
    // Acquire semaphore slot from sem (blocks if at max_concurrent)
    // On acquire: p.Send(SlotAcquiredMsg{SessionID: s.ID})
    //
    // Build args slice: ["--print", "--output-format", "stream-json", "--session-id", id]
    // If s.CompletedOnce: append "--resume"
    // If config.AllowedTools is non-empty: append "--allowedTools", strings.Join(tools, ",")
    // Append prompt as LAST positional argument
    //
    // exec.CommandContext(ctx, "claude", args...) — ctx carries timeout + shutdown
    // Call configureProcAttr(cmd) to set platform-specific process group isolation
    // Call configureProcCancel(cmd) to set platform-specific process tree kill on cancel
    // Set cmd.WaitDelay = 2 * time.Second for force-kill fallback
    //
    // Read stdout via pipe, line by line (stream-json):
    //   - Parse each line as JSON
    //   - On tool_use events: p.Send(StatusHintMsg{SessionID: s.ID, Hint: toolName})
    //   - Accumulate assistant text from assistant events
    //   - Cap accumulated output at 1MB; if exceeded, stop accumulating and append truncation notice
    //
    // Capture stderr separately
    // On completion: return SendResult{SessionID, Output, Err, CompletedAt}
    // DO NOT mutate session state here — Update handler does that
}
```

#### Platform-Specific Process Management

Process group isolation and cleanup differ between Unix and Windows. This logic is split into build-tagged files:

**`internal/session/proc_unix.go`** (`//go:build !windows`):

```go
package session

import (
    "os/exec"
    "syscall"
)

// ProcState holds platform-specific process management state.
// On Unix, no extra state is needed (process groups are implicit).
type ProcState struct{}

// configureProcAttr isolates the subprocess into its own process group.
func configureProcAttr(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// configureProcCancel sets cmd.Cancel to SIGTERM the entire process group (negative PID).
// This kills child processes spawned by claude (tool use, bash commands), not just the parent.
func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
    cmd.Cancel = func() error {
        return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
    }
}

// afterStart is called after cmd.Start(). On Unix, this is a no-op.
func afterStart(cmd *exec.Cmd, ps *ProcState) error {
    return nil
}

// cleanupProc releases platform-specific resources after cmd.Wait().
// On Unix, this is a no-op.
func cleanupProc(ps *ProcState) {}
```

**`internal/session/proc_windows.go`** (`//go:build windows`):

```go
package session

import (
    "os/exec"
    "syscall"
    "unsafe"

    "golang.org/x/sys/windows"
)

// ProcState holds platform-specific process management state.
// On Windows, it stores the Job Object handle for process tree management.
type ProcState struct {
    jobHandle windows.Handle
}

// configureProcAttr creates the subprocess in a new process group and suspended,
// so it can be assigned to a Job Object before it spawns any children.
func configureProcAttr(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{
        CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED,
    }
}

// configureProcCancel sets cmd.Cancel to terminate the process tree via the
// Job Object handle stored in ProcState. The handle is written by afterStart()
// after cmd.Start(). If the job was never assigned (error during setup), falls
// back to cmd.Process.Kill().
func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
    cmd.Cancel = func() error {
        if ps.jobHandle != 0 {
            err := windows.TerminateJobObject(ps.jobHandle, 1)
            windows.CloseHandle(ps.jobHandle)
            ps.jobHandle = 0
            return err
        }
        return cmd.Process.Kill() // fallback: kill parent only
    }
}

// afterStart is called after cmd.Start(). On Windows, it creates a Job Object,
// assigns the suspended process to it, and resumes the process. Must be called
// before the process does any work. On error, the process is resumed without a
// job — children may orphan on cancel.
func afterStart(cmd *exec.Cmd, ps *ProcState) error {
    job, err := assignJobAndResume(cmd)
    ps.jobHandle = job
    return err
}

// cleanupProc releases platform-specific resources after cmd.Wait().
// On Windows, closes the Job Object handle if still open.
func cleanupProc(ps *ProcState) {
    if ps.jobHandle != 0 {
        windows.CloseHandle(ps.jobHandle)
        ps.jobHandle = 0
    }
}

// assignJobAndResume creates a Job Object, assigns the suspended process to it,
// and resumes the process. Returns the Job Object handle or an error. On error,
// the process is resumed without a job — children may orphan on cancel.
func assignJobAndResume(cmd *exec.Cmd) (windows.Handle, error) {
    // Create a Job Object to track the entire process tree
    job, err := windows.CreateJobObject(nil, nil)
    if err != nil {
        resumeProcess(cmd)
        return 0, err
    }

    // Configure job to terminate all processes when the handle is closed
    info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
    info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
    _, err = windows.SetInformationJobObject(
        job,
        windows.JobObjectExtendedLimitInformation,
        uintptr(unsafe.Pointer(&info)),
        uint32(unsafe.Sizeof(info)),
    )
    if err != nil {
        windows.CloseHandle(job)
        resumeProcess(cmd)
        return 0, err
    }

    // Open a handle to the process and assign it to the job BEFORE resuming
    handle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
    if err != nil {
        windows.CloseHandle(job)
        resumeProcess(cmd)
        return 0, err
    }
    err = windows.AssignProcessToJobObject(job, handle)
    windows.CloseHandle(handle)
    if err != nil {
        windows.CloseHandle(job)
        resumeProcess(cmd)
        return 0, err
    }

    // Now resume the process — all children it spawns will inherit the job
    resumeProcess(cmd)
    return job, nil
}

// resumeProcess resumes a suspended process by finding its main thread via
// CreateToolhelp32Snapshot and calling ResumeThread. cmd.Process.Pid is a
// process ID, NOT a thread ID — OpenThread requires a thread ID, so we must
// enumerate threads to find one belonging to this process.
func resumeProcess(cmd *exec.Cmd) {
    pid := uint32(cmd.Process.Pid)
    snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
    if err != nil {
        return
    }
    defer windows.CloseHandle(snapshot)

    var entry windows.ThreadEntry32
    entry.Size = uint32(unsafe.Sizeof(entry))
    err = windows.Thread32First(snapshot, &entry)
    for err == nil {
        if entry.OwnerProcessID == pid {
            threadHandle, terr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
            if terr == nil {
                windows.ResumeThread(threadHandle)
                windows.CloseHandle(threadHandle)
            }
            return
        }
        err = windows.Thread32Next(snapshot, &entry)
    }
}
```

**Why Job Objects at creation time:** On Windows, there is no process group signal equivalent to `kill(-pid, SIGTERM)`. Job Objects are the standard mechanism for managing process trees. The process must be assigned to the Job Object **before it starts running** (using `CREATE_SUSPENDED` + assign + resume) so that all child processes it spawns inherit the job membership. If the job were assigned at cancel time (as in an earlier version of this spec), children spawned before cancellation would escape the job and be orphaned. The fallback to `cmd.Process.Kill()` ensures cancel never fails entirely, though orphaned children are possible in the fallback path.

**Usage in Send():** The platform-specific logic is abstracted behind four functions with identical signatures on both platforms: `configureProcAttr(cmd)`, `configureProcCancel(cmd, ps)`, `afterStart(cmd, ps)`, and `cleanupProc(ps)`. The `ProcState` type is defined per-platform (empty struct on Unix, contains Job Object handle on Windows). This keeps `Send()` fully cross-platform:

```go
// In Send() (cross-platform session.go):
var ps ProcState
configureProcAttr(cmd)
configureProcCancel(cmd, &ps)  // sets up Cancel to use ps (on Windows, reads jobHandle via pointer)
cmd.Start()
afterStart(cmd, &ps)           // Unix: no-op. Windows: assigns Job Object + resumes suspended process.
defer cleanupProc(&ps)         // Unix: no-op. Windows: closes Job Object handle if still open.
// ... read stdout, wait for completion ...
```

**Timeout placement:** The timeout context is created in the `handleSend()` function (on the main goroutine), **before** launching the goroutine that calls `Send()`. This ensures the timeout starts from when the user presses Enter, not from when the semaphore is acquired. If all subprocess slots are full, the timeout counts down while waiting for a slot — preventing unbounded waits.

```go
// In handleSend():
timeout := config.SubprocessTimeout
ctx, cancel := context.WithTimeout(m.RootCtx, timeout)
active.CancelFunc = cancel  // Store so dismiss handler (Ctrl+W) can cancel this subprocess
sem := m.Sem
m.ShutdownWg.Add(1)
go func() {
    defer m.ShutdownWg.Done()
    defer cancel()
    result := active.Send(ctx, prompt, sem, p)
    p.Send(SessionDoneMsg{Result: result})
}()
```

### Queue.Rebuild()

```go
func (q *Queue) Rebuild(sessions []*Session) {
    // Filter to NeedsInput only
    // Use slices.SortStableFunc to sort by sortKey ascending (oldest first = FIFO), where:
    //   sortKey = SkippedAt if non-zero (card was skipped), else EnteredQueue
    // Stable sort preserves insertion order for sessions with identical timestamps,
    // preventing UI flicker when multiple sessions complete within the same millisecond.
    // Tiebreaker: if sortKeys are equal, preserve original slice order (stable sort handles this).
    //
    // This ensures skipped cards go to the end of the queue without resetting
    // the user-visible "waiting Xm" timer (which reads EnteredQueue).
    //
    // Note on new arrivals after skip: If card A is skipped at time T, then card B
    // finishes and enters the queue at T+5, card B sorts AFTER card A (T+5 > T).
    // This means card A — despite being skipped — appears before newly arrived cards.
    // This is intentional: skip sends the card to the end of the queue at the moment
    // of skipping, but new arrivals that come in later naturally sort after it.
    // The sort is purely chronological by the relevant timestamp.
}
```

### Mock Mode

When `claude` CLI is not in PATH, or `--mock` flag is passed, sessions use `MockSend()` instead of `Send()`. MockSend sleeps for 2-8 seconds randomly, then returns a random response from a hardcoded list of realistic Claude-style outputs (questions, option lists, completion messages). This allows full UI development and testing without Claude Code installed.

Auto-detection on startup:
```go
if _, err := exec.LookPath("claude"); err != nil {
    mockMode = true
    // Print warning before entering alt screen
}
```

---

## 9. Safety & Limits

### 9.1 Input Sanitization

#### Session Names

Session names are used in `--session-id` flags and displayed in the UI. They must be validated on creation (both CLI args and Ctrl+N overlay):

- **Auto-lowercase:** Input is lowercased before validation (e.g. `"Auth"` becomes `"auth"`). This applies to both CLI args and the Ctrl+N overlay.
- **Allowed characters:** `a-z`, `0-9`, `-`, `_` (lowercase alphanumeric plus dash and underscore).
- **First character:** Must be alphanumeric (`a-z` or `0-9`). Names starting with `-` or `_` are rejected to avoid flag-parsing ambiguity and filesystem edge cases.
- **Max length:** 32 characters.
- **Rejection:** If a name from CLI args fails validation after lowercasing, fall back to an auto-generated name (`s1`, `s2`, etc.) and print a warning to stderr (e.g. `Warning: invalid session name "a!b", using "s3"`). If a name from the Ctrl+N overlay fails, show an inline error and keep the overlay open.
- **Why:** The name is interpolated into `--session-id deck-{name}-{shortid}`. Unrestricted characters could conflict with CLI flag parsing (e.g. a name starting with `--`), break terminal rendering (control characters), or cause unexpected behavior in the claude CLI's session storage.

#### Prompt Text

Prompts are passed as the final positional argument to `exec.Command("claude", args...)`. Because `exec.Command` does **not** invoke a shell, there is no shell injection risk. However:

- **Positional argument placement:** The prompt must always be the last argument. Build the args slice explicitly — never concatenate into a single string.
- **Empty prompts:** Reject empty prompts. If the user presses Enter with no text, do nothing (already specified in key bindings).

#### Claude Output Sanitization

Claude's response text is displayed in the TUI. Before rendering, strip or escape:

- **ANSI escape sequences:** Strip all CSI sequences (`\x1b[...`), OSC sequences (`\x1b]...`), and other escape codes. Use a library like `github.com/acarl005/stripansi` or equivalent.
- **Control characters:** Strip characters in `0x00-0x1F` range except `\n` (newline) and `\t` (tab). These can corrupt terminal state.
- **Why:** Claude's output could contain escape sequences (from code blocks, tool output, etc.) that disrupt the TUI layout or, in legacy terminals, perform unintended actions.

### 9.2 Subprocess Management

#### Concurrency Limit

Maximum concurrent subprocesses: **5** (configurable via `max_concurrent` in config). When the limit is reached, new `Send()` calls queue internally and dispatch when a slot opens. The session transitions to Working immediately (so the UI is responsive) but the actual subprocess launch is gated.

Implementation: use a buffered channel of size `max_concurrent` as a semaphore, stored as a field on the `Model` struct (initialized after config is loaded at startup).

```go
// In Model struct:
//   Sem chan struct{}   // initialized in NewModel: make(chan struct{}, config.MaxConcurrent)
// The semaphore is passed to Send() or accessed via a shared reference — NOT a package-level var,
// since config.MaxConcurrent is not known until runtime.

// sem is passed from Model.Sem (initialized after config load as make(chan struct{}, config.MaxConcurrent))
func (s *Session) Send(ctx context.Context, prompt string, sem chan struct{}, p *tea.Program) SendResult {
    // Acquire slot with context awareness — timeout can cancel the wait
    select {
    case sem <- struct{}{}:
        defer func() { <-sem }()
        p.Send(SlotAcquiredMsg{SessionID: s.ID})
    case <-ctx.Done():
        return SendResult{SessionID: s.ID, Err: ctx.Err(), CompletedAt: time.Now()}
    }
    // ... build command, call configureProcAttr/configureProcCancel, exec ...
}
```

#### Subprocess Timeout

Each subprocess has a timeout of **10 minutes** (configurable via `subprocess_timeout` in config). The timeout context is created in `handleSend()` **before** spawning the goroutine (see Section 8, Session.Send()), so the clock starts from when the user presses Enter — not from when the semaphore is acquired. This prevents unbounded waits when slots are full.

Inside `Send()`, the context is passed to `exec.CommandContext` with Go 1.20+ `cmd.Cancel` / `cmd.WaitDelay` for graceful shutdown:

```go
// ctx is created in handleSend() with the timeout already ticking
cmd := exec.CommandContext(ctx, "claude", args...)
var ps ProcState
configureProcAttr(cmd)        // platform-specific process group isolation
configureProcCancel(cmd, &ps) // platform-specific process tree kill (reads ProcState via pointer)
cmd.WaitDelay = 2 * time.Second
// After cmd.Start(): afterStart(cmd, &ps) sets up Windows Job Object; no-op on Unix.
// After cmd.Wait(): cleanupProc(&ps) releases platform resources.
```

When a subprocess times out:
- **Unix:** SIGTERM is sent to the process group first, giving processes 2 seconds to clean up. If not exited after 2 seconds, SIGKILL is sent automatically by `cmd.WaitDelay`.
- **Windows:** The Job Object terminates all processes in the tree immediately. `cmd.WaitDelay` provides the force-kill fallback via `Process.Kill()` if the cancel function fails.
- The session transitions to Needs You with the error: `Timed out after {duration}`.
- The user can retry by sending another message.
- If the timeout fires while the goroutine is waiting for a semaphore slot (not yet launched), the context cancellation unblocks the semaphore acquire (use `select` on `ctx.Done()` alongside the channel send) and the session transitions to Needs You with: `Timed out waiting for subprocess slot`.

#### Output Size Cap

Subprocess stdout is read line-by-line via a pipe (for stream-json parsing). The accumulated assistant response text is capped at **1 MB**. Once the cap is reached, further assistant text events are discarded and a truncation notice is appended: `\n\n[output truncated at 1MB]`. Stderr is captured separately via a buffer. The cap applies to the assembled response text, not to the raw stream-json bytes (which include metadata that is not stored).

```go
// In Send(), reading from stdout pipe:
scanner := bufio.NewScanner(stdout)
// Increase scanner buffer from default 64KB to 4MB. Claude's stream-json can emit
// large single-line events (e.g. full file contents in tool results). The default
// 64KB limit causes bufio.ErrTooLong which silently stops reading all remaining output.
buf := make([]byte, 0, 64*1024)
scanner.Buffer(buf, 4*1024*1024)
var output strings.Builder
for scanner.Scan() {
    event := parseStreamJSON(scanner.Text())
    if event.IsAssistantText && output.Len() < 1_000_000 {
        output.WriteString(event.Text)
    }
    // ... handle other event types (tool_use → StatusHintMsg, etc.)
}
// Check for scanner errors. bufio.ErrTooLong means a single line exceeded the 4MB
// buffer — all remaining output after that line was lost. This is a known limitation;
// a line >4MB would require a streaming JSON parser instead of line-based scanning.
// Log the error if verbose mode is enabled, but do not fail the session — return
// whatever output was accumulated before the error.
if err := scanner.Err(); err != nil {
    if verbose { log.Printf("scanner error for session %s: %v", s.ID, err) }
    if output.Len() > 0 {
        output.WriteString("\n\n[output may be incomplete — stream read error]")
    }
}
```

#### Graceful Shutdown

If any sessions are in Working state, the first Ctrl+C shows a notification: `{N} sessions running — Ctrl+C again to quit`. A second Ctrl+C within 1 second proceeds with shutdown. If no sessions are Working, a single Ctrl+C quits immediately.

**Bubbletea Ctrl+C interception:** Bubbletea's default behavior quits the program on Ctrl+C. To implement the double-Ctrl+C logic, the Update handler must intercept `tea.KeyCtrlC` and conditionally return `tea.Quit` only on the second press. Do NOT use `tea.WithoutSignalHandler()` — instead, handle `tea.KeyCtrlC` as a regular key message in Update:

```go
case tea.KeyMsg:
    if msg.Type == tea.KeyCtrlC {
        if noWorkingSessions || (withinConfirmWindow) {
            return m, tea.Quit
        }
        // Show warning, start confirm window
        return m, nil
    }
```

On application exit (Ctrl+C confirmed, or OS-level termination signal — SIGTERM/SIGINT/SIGHUP on Unix, console close on Windows):

1. Cancel all subprocess contexts (triggers platform-specific process tree kill via `cmd.Cancel`, then force-kill after 2s via `cmd.WaitDelay`).
2. Wait up to 3 seconds for goroutines to return.
3. Exit. Do not wait indefinitely — if a subprocess is stuck, force exit.

Implementation: create a root context and pass it to the Bubbletea program via `tea.WithContext(ctx)`. Note that `tea.WithContext` does **not** cancel the provided context when the program exits — it only uses the context to detect external cancellation. The `defer cancel()` in `main()` is what actually cancels `RootCtx` after `p.Run()` returns, triggering subprocess cleanup.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// ctx is the SAME context stored as model.RootCtx. Subprocess contexts are derived
// from model.RootCtx, so when cancel() fires after p.Run() returns, all subprocesses
// receive cancellation automatically.
model.RootCtx = ctx
model.RootCancel = cancel
p := tea.NewProgram(model, tea.WithContext(ctx))
if _, err := p.Run(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}
cancel() // Cancel all subprocess contexts (also fired by defer, but explicit here for clarity)

// Wait for goroutines to finish cleanup, with a hard deadline
done := make(chan struct{})
go func() {
    model.ShutdownWg.Wait()
    close(done)
}()
select {
case <-done:
    // All goroutines exited cleanly
case <-time.After(3 * time.Second):
    // Force exit — stuck subprocesses are abandoned
}
```

Each subprocess goroutine must increment/decrement the WaitGroup:

```go
// In handleSend():
active.CancelFunc = cancel  // Store so dismiss handler (Ctrl+W) can cancel this subprocess
m.ShutdownWg.Add(1)
go func() {
    defer m.ShutdownWg.Done()
    defer cancel()
    result := active.Send(ctx, prompt, sem, p)
    p.Send(SessionDoneMsg{Result: result})
}()
```

### 9.3 Concurrency Safety

`Session.Send()` runs in a goroutine and mutates session state. The Bubbletea Update/View loop reads session state on the main goroutine. This is a data race.

**Solution:** Session state mutations from goroutines must not write to shared fields directly. Instead, the goroutine sends a `SessionDoneMsg` through the Bubbletea event system, and the `Update` function (which runs on the main goroutine) applies the state changes.

The `Send()` function must **not** modify `s.State`, `s.EnteredQueue`, or `s.History` directly. Instead, it returns a `SendResult` containing the output, error, and timing. The `Update` handler applies these:

```go
// In the goroutine (Send):
// DO NOT write to s.State or s.History here.
// Communicate via Bubbletea messages only:
//   - p.Send(SlotAcquiredMsg{...})    → on semaphore acquire
//   - p.Send(StatusHintMsg{...})      → on stream-json activity events
//   - p.Send(SessionDoneMsg{...})     → on completion (returned as SendResult)
return SendResult{SessionID: s.ID, Output: output, Err: err, CompletedAt: time.Now()}

// In Update (main goroutine):
// NOTE: All message handlers must nil-check the session lookup result.
// A dismissed session's goroutine may still deliver messages after removal.
case SlotAcquiredMsg:
    s := findSession(msg.SessionID)
    if s == nil { break }  // session was dismissed while goroutine was in flight
    s.SlotAcquired = true

case StatusHintMsg:
    s := findSession(msg.SessionID)
    if s == nil { break }
    s.StatusHint = msg.Hint

case SessionDoneMsg:
    s := findSession(msg.Result.SessionID)
    if s == nil { break }  // session was dismissed; discard the result
    s.State = NeedsInput
    s.SlotAcquired = false
    s.CancelFunc = nil       // subprocess finished; no context to cancel
    if msg.Result.Err == nil {
        s.CompletedOnce = true  // only on success — failed first messages must retry without --resume
    }
    s.StatusHint = ""
    s.EnteredQueue = msg.Result.CompletedAt
    s.History = append(s.History, Message{Role: "claude", Text: msg.Result.Output})
```

### 9.4 Config File Safety

#### File Permissions

On first-run creation:
- Resolve config directory via `os.UserConfigDir()` (see Section 7 for platform-specific paths).
- Create directory `{configDir}/claude-deck/` with mode `0700`. On Windows, `os.MkdirAll` ignores Unix permission bits; the default Windows ACLs restrict access to the creating user and administrators, which is equivalent.
- Create `config.json` with mode `0600`. Same Windows caveat applies — the file inherits restrictive ACLs from the parent directory.

On load:
- If the file exists but is not parseable JSON, log a warning to stderr (before entering alt screen) and use defaults.
- If the file has unexpected field types (e.g. string where int expected), ignore the invalid field and use its default.
- If the file is not readable (permission denied), log a warning and use defaults.

Never crash on config errors. Always fall back to defaults with a visible warning.

#### Validation

| Field | Validation | On invalid |
|-------|-----------|------------|
| `default_reply` | Non-whitespace string (must contain at least one non-space character), max 500 chars | Use default |
| `focus_threshold` | Integer 1-50 | Use default |
| `bell_on_queue` | Boolean | Use default |
| `max_concurrent` | Integer 1-20 | Use default |
| `subprocess_timeout` | Go duration string (`time.ParseDuration` format), 30s-30m | Use default |
| `allowed_tools` | Array of strings, each non-empty | Use default (empty = unrestricted) |

### 9.5 Session Limits

- **Maximum total sessions:** 20. Attempting to create more shows a notification: `Session limit reached (20)`. This prevents unbounded process creation.
- **Maximum conversation history per session:** 200 messages. After 200 messages, the oldest messages are dropped from the in-memory history (the claude CLI's `--resume` still has full context via its own session storage). Show a muted indicator: `(earlier messages trimmed)`.
- **Memory budget:** With 20 sessions × 200 messages × up to 1MB output cap, the theoretical maximum is ~4GB. In practice this is unlikely (most Claude responses are far smaller), but the 1MB output cap per subprocess (Section 9.2) and the 200-message trim per session bound the worst case. No additional memory pressure handling is needed for MVP, but this should be monitored.

### 9.6 Tool Use Safety

**Critical:** In `--print` mode, Claude Code auto-accepts all tool calls — file writes, shell commands, everything. With multiple concurrent sessions, this means multiple instances of Claude Code making unsupervised modifications to the filesystem simultaneously. This is the most significant safety consideration in claude-deck.

**Mitigations:**

1. **`--allowedTools` config option:** The `allowed_tools` config field (see Section 7) accepts a list of tool names. When non-empty, each subprocess is launched with `--allowedTools {tools}`, restricting Claude to only the specified tools. Example conservative configuration:
   ```json
   {
     "allowed_tools": ["Read", "Glob", "Grep", "WebSearch", "WebFetch"]
   }
   ```
   This makes sessions read-only — Claude can research and answer questions but cannot modify files or run commands. Users who want write access can add `"Edit"`, `"Write"`, `"Bash"` etc. to the list explicitly.

2. **Startup warning:** On startup (when not in mock mode and `allowed_tools` is empty), print a warning to stderr before entering the alt screen:
   ```
   Warning: Sessions run with full tool access (--print mode auto-accepts all tool calls).
   Configure "allowed_tools" in your config file to restrict (see Section 7 for path).
   ```

3. **Concurrent file conflict risk:** Two sessions may attempt to edit the same file simultaneously, causing data loss or corruption. claude-deck does **not** attempt to prevent this — it is the user's responsibility to ensure sessions work on non-overlapping files, or to restrict tool access via `allowed_tools`. A future version may add file-level locking or working directory isolation per session.

---

## 10. Rendering Details

### Color Palette

| Name | Hex | Usage |
|------|-----|-------|
| Amber | `#f59e0b` | Needs You badges, queue indicators, focus mode accent |
| Blue | `#3b82f6` | Working badges, spinner |
| Brand | `#d4a574` | App name, claude role prefix, active input border |
| Fg | `#e5e5e5` | Primary text |
| Muted | `#737373` | Secondary text, hints |
| Dim | `#404040` | Borders, inactive elements |
| Bg | `#0a0a0a` | Background |
| Surface | `#151515` | Card backgrounds |
| Red | `#ef4444` | Errors |
| Green | `#22c55e` | Success (reserved) |
| UserBlue | `#60a5fa` | User message text |
| UserBg | `#1e3a5f` | User message background (chat bubbles) |
| ClaudeBg | `#2d1f0e` | Claude role indicator background |

### Text Wrapping

All text content is word-wrapped to `terminalWidth - 6` (3 chars padding each side). Long words that exceed the available width are broken at the width boundary.

### Card Width

Each card in the strip is 24 characters wide. The `+N` overflow indicator is 4 characters wide (e.g. `+12`). Maximum visible cards = `(terminalWidth - 4 - overflowWidth) / 26` (24 + 2 for gap), where `overflowWidth` is 0 when all cards fit or 4 when overflow is needed. Calculate in two passes: first check if all cards fit in `(terminalWidth - 4) / 26`; if not, recalculate with the overflow indicator width reserved.

### Minimum Terminal Size

Minimum usable: 60 columns × 16 rows. Below this, show a message asking to resize. The app should degrade gracefully — when card strip is visible it can shrink to 1 card, chat view to 3 lines.

---

## 11. Edge Cases

### Focus Mode Entry with Empty Queue
Pressing `F` when the queue has 0 items is a no-op. Show a transient notification: `No cards in queue`.

### Queue Empty in Focus Mode
Show centered waiting screen with spinner and count of working sessions. When a new card arrives, immediately display it. If `len(Sessions) == 0` (all sessions have been dismissed) and the queue is empty, auto-exit focus mode.

### All Sessions Working in Normal Mode
If card strip is visible, it shows all cards with blue Working badges. Queue nav bar is hidden. Chat view shows the most recently active session's history (read-only, where "most recently active" means the session the user most recently sent a message to). Input is disabled with placeholder "Claude is working...".

### Rapid Queue Fills
If multiple sessions finish simultaneously, they enter the queue in the order their subprocesses completed (effectively random). Notifications stack and each auto-dismiss independently after 3s.

### Very Long Claude Output
Chat view scrolls. In Focus Mode, long output fills the available vertical space and can be scrolled. The viewport component handles this.

### Session Error
If `claude` subprocess fails (non-zero exit, timeout, context cancelled), session enters Needs You with the error as the last output. User can retry by sending another message. Common errors: `claude` not in PATH (caught at startup), `claude` not executable (permission denied), network failure, rate limiting, subprocess timeout (10m default).

### Subprocess Timeout
When a subprocess exceeds the configured timeout (counting from when the user pressed Enter, including any time spent waiting for a semaphore slot), it is killed and the session transitions to Needs You with the message: `Timed out after {duration}. Send another message to retry.` If the timeout fires while waiting for a slot, the message is: `Timed out waiting for subprocess slot.`

### Max Sessions Reached
When the user tries to create a 21st session (via CLI args or Ctrl+N), show a notification: `Session limit reached (20)`. For CLI args, the **first 20 arguments** are accepted and the rest are skipped with a stderr warning before entering the alt screen (e.g. `Warning: session limit is 20, skipping 3 extra arguments`).

### Max Concurrent Subprocesses
When all subprocess slots are occupied, new sends queue internally. The session shows Working state with a **dim spinner** (see Section 2.1) to indicate it is waiting for a slot. When a slot opens, a `SlotAcquiredMsg` is sent, the spinner switches to the normal blue style, and the subprocess launches. The timeout is already ticking during the slot wait (see Section 9.2).

### Empty State (0 Sessions)
When there are no sessions (app launched with no arguments and no sessions created yet), show a centered message: `Press Ctrl+N to create a session`. The card strip, queue nav, chat view, and input bar are all hidden. Only the top bar and the centered prompt are visible.

### Terminal Resize
Bubbletea sends WindowSizeMsg. All rendering recalculates based on new dimensions. Card strip (if visible) adjusts visible count. Chat view adjusts line count. Word wrapping recalculates.

### Resume After Failed First Message
If the first subprocess for a session is killed (timeout, Ctrl+W, context cancellation) before completing, the `CompletedOnce` flag remains false. The next `Send()` call omits `--resume`, effectively retrying as a fresh first message. This avoids `--resume` failing on a session ID that Claude CLI never persisted.

### Claude CLI Not Executable
If `claude` is found in PATH by `exec.LookPath` but fails with a permission error when executed, the session transitions to Needs You with the error message. The user can fix permissions and retry by sending another message. This is distinct from the startup check (which only verifies PATH presence and version).

### Concurrent File Edits
Two sessions may attempt to edit the same file simultaneously. claude-deck does not prevent this. The last write wins, and earlier changes may be silently lost. Users should either structure prompts to target non-overlapping files, or configure `allowed_tools` to restrict write access. See Section 9.6.

---

## 12. Future Considerations (Not in MVP)

These are documented for context but should **not** be built in the first version:

- **Session persistence:** Save session IDs to disk so they survive app restart. Resume with `claude --resume --session-id`.
- **Session groups/tags:** Organize sessions by project or concern.
- **Custom queue ordering:** Priority-based or manual reorder (conflicts with FIFO simplicity).
- **Split view:** Show 2 queue items side-by-side for comparison (conflicts with card philosophy).
- **Prompt templates:** Predefined replies beyond the single default_reply.
- **Session forking:** Branch a conversation into two sessions from a decision point.
- **Stats/analytics:** Track response times, session duration, prompts per session.
- **Multiplayer:** Multiple users working the same queue (needs a server).
- **Native clipboard integration:** Copy Claude's output to system clipboard.
- **Full streaming output:** Show Claude's complete response as it streams in the chat view (real-time text appearing in the conversation). The current MVP uses `--output-format stream-json` for status hints only (tool names, activity indicators) but does not render incremental response text. Full streaming would show text appearing word-by-word in the chat view.

---

## 13. Build & Run

```bash
# Build (Unix/macOS)
go build -o claude-deck .

# Build (Windows)
go build -o claude-deck.exe .

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o claude-deck .
GOOS=windows GOARCH=amd64 go build -o claude-deck.exe .
GOOS=darwin GOARCH=arm64 go build -o claude-deck .

# Run with sessions
claude-deck "auth:refactor auth middleware to use JWT" \
            "tests:write integration tests for /api/v2" \
            "db:normalize the users table"

# Run in mock mode (no claude CLI needed)
claude-deck --mock "demo:do something cool"

# Run empty, create sessions interactively
claude-deck

# Run with verbose logging (writes to {configDir}/claude-deck/debug.log)
claude-deck --verbose "auth:refactor auth middleware"
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--mock` | Use mock subprocess instead of real claude CLI |
| `--verbose` | Enable debug logging to `{configDir}/claude-deck/debug.log` (where `configDir` is the OS-standard config directory — see Section 7). Logs subprocess launches, exits, semaphore acquire/release, config loading, stream-json parse events, and errors. The log file is truncated on each launch. Useful for diagnosing session failures, timeout issues, and stream parsing problems. |

### Dependencies

- Go 1.22+
- `github.com/charmbracelet/bubbletea` v0.25+ — TUI framework (v0.25+ required for `tea.WithContext`)
- `github.com/charmbracelet/lipgloss` — Terminal styling
- `github.com/charmbracelet/bubbles` — Text input, viewport components
- `github.com/google/uuid` — Session ID generation
- `golang.org/x/sys` — Windows Job Object APIs (only imported on Windows via build tags)

### Runtime Requirement

- Claude Code CLI (`claude`) **version 1.0.0+** in PATH — or use `--mock` flag for testing. The `--print --resume --session-id` flag combination requires 1.0.0 or later. On Windows, `claude` must be callable via `exec.LookPath` (typically installed globally via npm and available as `claude.cmd` or `claude.exe`).
- On startup (when not in mock mode), run `claude --version` and parse the output. The output format varies (e.g. `1.2.3`, `claude 1.2.3`, `Claude Code v1.2.3`). Extract the first semver-like string by matching the regex `(\d+\.\d+\.\d+)` from stdout. Compare the major version (>= 1). If the version is below 1.0.0, no match is found, or the command fails, print a warning to stderr before entering the alt screen: `Warning: claude CLI version 1.0.0+ required (found: {version})`.

### Platform Support

claude-deck builds and runs on **Linux**, **macOS**, and **Windows**. Platform-specific behavior:

| Concern | Unix/macOS | Windows |
|---------|-----------|---------|
| Process tree kill | `Setpgid` + `kill(-pid, SIGTERM)` | Windows Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (assigned at process creation via `CREATE_SUSPENDED` + assign + resume) |
| Config directory | `os.UserConfigDir()` → `~/.config` (Linux), `~/Library/Application Support` (macOS) | `os.UserConfigDir()` → `%APPDATA%` |
| File permissions | `0700`/`0600` enforced by OS | Permission bits ignored; Windows ACLs provide equivalent restriction |
| Graceful shutdown signals | SIGTERM, SIGINT, SIGHUP | Console close event (handled by Go runtime + Bubbletea) |
| `claude` CLI binary | `claude` | `claude.cmd` or `claude.exe` (resolved by `exec.LookPath`) |

All platform-specific code is isolated in `internal/session/proc_unix.go` and `internal/session/proc_windows.go` using Go build tags. The rest of the codebase is fully cross-platform.
