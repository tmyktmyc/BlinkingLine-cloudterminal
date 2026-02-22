# Deck Mode UX Redesign

> Replace Normal Mode + Focus Mode + Welcome Screen with a single unified "Deck Mode" that auto-switches between sessions across multiple projects.

## Problem

The current UX has three issues:

1. **Ctrl+N is intercepted by Terminal.app** — the "new session" shortcut opens a macOS window instead of a CloudTerminal session.
2. **The welcome screen is confusing** — users launch CloudTerminal and see instructions but don't know what to do. Creating a session opens what looks like a blank terminal.
3. **Two modes (Normal + Focus) add cognitive overhead** — users shouldn't have to learn mode-switching to use the tool.

## Design: The Deck of Cards

Sessions are a stack of cards. CloudTerminal always shows **one card at a time**. The system decides which card is on top based on priority:

1. **Sessions needing input** — shown first (FIFO, skipped ones go to back)
2. **Sessions the user manually browses to** — via arrow keys
3. **Live status feed** — when nothing needs input, shows all sessions' real-time activity

After you reply to a card, it goes to the bottom of the deck (working state), and the next card needing input automatically surfaces. If no cards need input, the live status feed appears.

### Card View (Session Needs Input)

```
┌────────────────────────────────────────────────┐
│  auth  ●  ~/code/backend           2/4 waiting │
│                                                │
│  I've refactored the auth middleware to use    │
│  JWT. Should I also update the refresh flow?   │
│                                                │
│────────────────────────────────────────────────│
│  ▸ yes, update the refresh flow too            │
│                                   ? for help   │
└────────────────────────────────────────────────┘
```

- **Header:** session name, state badge, project directory (shortened), queue position
- **Body:** Claude's last message (scrollable with Ctrl+U/D)
- **Input bar:** reply text, with `? for help` hint

### Live Status Feed (Nothing Needs Input)

```
┌────────────────────────────────────────────────┐
│  ◆ CloudTerminal                  4 working    │
│                                                │
│  auth      ⟳ editing auth.ts          (45s)   │
│  tests     ⟳ running tests            (1m)    │
│  refactor  ⟳ reading files            (12s)   │
│  db        ⟳ waiting for slot         (2s)    │
│                                                │
│────────────────────────────────────────────────│
│  ▸ /new, /list, /skip, /dismiss  ? for help   │
│                                                │
└────────────────────────────────────────────────┘
```

- Shows each session's name, spinner, status hint (from stream-json parsing), and elapsed time
- Updates live as status hints change
- Input bar shows command hints

### Empty State (No Sessions)

```
┌────────────────────────────────────────────────┐
│  ◆ CloudTerminal                               │
│                                                │
│  Type /new to start a session, ? for help      │
│                                                │
│────────────────────────────────────────────────│
│  ▸                                             │
└────────────────────────────────────────────────┘
```

One line. No tutorial wall.

## Command System

All interaction through the input bar. Commands start with `/`.

| Command | Action |
|---------|--------|
| `/new` | Create session (step-by-step: name → dir → prompt) |
| `/list` | Show all sessions with status |
| `/skip` | Skip current card to back of deck |
| `/dismiss` | Close current session (double-type for working sessions) |
| `/go <name>` | Jump to a specific session by name |
| `?` | Show available commands and keybindings |
| ← → | Browse sessions manually (when input empty) |
| Enter | Send reply to current session |
| Esc | Cancel current `/new` flow or return to priority view |

### `/new` Flow (Inline, Step-by-Step)

When the user types `/new` and presses Enter:

```
  Name: auth
```
User types name, presses Enter.
```
  Name: auth
  Dir [.]: ~/code/backend
```
User types directory (tab-completion, Enter accepts default = cwd).
```
  Name: auth
  Dir: ~/code/backend
  Prompt: refactor the auth module to use JWT
```
User types prompt, presses Enter. Session created and dispatched.

- Each step shown inline in the input area
- Esc cancels at any point, returns to previous view
- Invalid input shows inline error and re-prompts

### `?` Help Output

Shown inline, temporarily replacing the card view:

```
  Commands:
    /new              Create a new session
    /list             Show all sessions
    /skip             Skip current to back of queue
    /dismiss          Close current session
    /go <name>        Jump to session by name

  Navigation:
    ← →               Browse sessions
    Ctrl+U / Ctrl+D   Scroll chat
    Ctrl+C Ctrl+C     Quit

  Press any key to return.
```

## Multi-Project Sessions

Each session carries a **working directory**:

```go
type Session struct {
    Name    string
    Dir     string    // Working directory for this session
    State   SessionState
    // ... rest unchanged
}
```

Claude CLI invocation uses the session's directory:

```go
cmd := exec.CommandContext(ctx, "claude",
    "--print",
    "--output-format", "stream-json",
    "--session-id", session.ID,
    prompt,
)
cmd.Dir = session.Dir
```

### CLI Args Update

Command-line session specs gain an optional directory prefix:

```
cloudterminal "auth:~/code/backend:refactor auth" "ui:~/code/frontend:fix navbar"
```

Format: `name:dir:prompt` (if two colons present) or `name:prompt` (existing format, uses cwd).

## Auto-Switch Behavior

### After Sending a Reply

1. Current session transitions to Working
2. Queue rebuilds
3. If queue has items → show next NeedsInput session (top of deck)
4. If queue empty → show live status feed

### When a Session Finishes (SessionDoneMsg)

1. Session transitions to NeedsInput
2. If user is on the live status feed → auto-switch to the new card
3. If user is manually browsing another session → notification appears, no forced switch
4. If user is replying (input has text) → notification only, no switch

### Manual Override

- Arrow keys let user browse to any session (including working ones)
- After browsing, replying to any session returns to auto-switch behavior
- `/go name` jumps directly to a session

## What Gets Removed

- **Welcome screen** — replaced with one-liner empty state
- **Normal Mode / Focus Mode distinction** — single Deck Mode
- **Card strip** — gone (single pane only)
- **Ctrl+N overlay** — replaced with `/new` inline command
- **Mode-specific keybindings (F, V, S)** — replaced by `/` commands
- **Alt+1-9, Ctrl+1-9 jump shortcuts** — replaced by `/go name`

## What Stays

- **Queue system** — FIFO with skip-to-back, drives the deck
- **Stream-json parsing** — status hints for live feed
- **Semaphore concurrency** — limits parallel subprocesses
- **Notification system** — auto-expiring, shown at top of card
- **Config file** — same location, same fields
- **Mock mode** — still works for testing
- **`--version` flag** — just added
- **Subprocess timeout, bell, allowed_tools** — unchanged

## Architecture Impact

### Files to Modify

| File | Change |
|------|--------|
| `internal/ui/model.go` | Remove Mode enum, add DeckState (Card/Feed/Help/NewSession), add Dir to session creation |
| `internal/ui/update.go` | Replace handleNormalKey/handleFocusKey with single handleDeckKey, add command parsing |
| `internal/ui/view.go` | Replace renderNormal/renderFocus with renderCard/renderFeed/renderHelp/renderNewFlow |
| `internal/ui/styles.go` | Simplify (remove strip styles, mode-specific styles) |
| `internal/session/session.go` | Add Dir field to Session struct |
| `internal/session/send.go` | Set cmd.Dir from session.Dir |
| `main.go` | Update parseSessionArgs to support name:dir:prompt format |

### Files to Remove

| File | Reason |
|------|--------|
| `internal/ui/overlay.go` | Replaced by inline `/new` flow |

### New Rendering Functions

- `renderCard(session)` — single session card with header, message, input
- `renderFeed(sessions)` — live status feed of all working sessions
- `renderHelp()` — command reference
- `renderNewFlow(step)` — inline step-by-step session creation
- `renderEmpty()` — one-line empty state

## Success Criteria

1. User launches CloudTerminal → sees one-liner, types `/new`, creates a session
2. Session runs, finishes, card auto-appears with Claude's response
3. User replies, next card auto-appears (or feed if nothing waiting)
4. User can create sessions in different project directories
5. Live feed shows real-time status of all working sessions
6. No mode confusion — one pane, one mental model
7. All existing tests still pass (adapted to new structure)
