# Docent guide

Docent shows folders of Markdown as a clean, searchable documentation site, on your own computer. Nothing is uploaded and nothing in your folders is changed.

## Start

```bash
docent                     # starts the app and opens it in your browser (http://127.0.0.1:8787)
docent add ~/notes         # remember a folder: it is there every time you start
docent open ~/Downloads/readme.md   # look at any folder or file once, without remembering it
```

In the app, **Open folder or file** (bottom of the left column) lets you pick a folder with a file browser, then asks whether to just open it or add it to your libraries.
A folder you add is called a *library*. Remove one from the home page; this only forgets it, your files stay.

Docent runs as one small program: no database, no account, no internet needed. It listens on your own computer only. Closing the browser tab does not stop it; press Ctrl+C in the terminal (on Windows, close the console window that opened with it), or see *Run it in the background* below.

## Reading

Press `?` at any time to come back to this guide.

| You want to | Do this |
| ----------- | ------- |
| Find a page | `Ctrl K` (or `/`), type, `Enter`. It searches titles and the text of every page; `Esc` closes. On a phone, tap the magnifier. |
| Find a note by its ID | In a developer-notes library, press `Ctrl K` and type the ID in any form: `DOC-0015`, `doc-15` or just `15`. The note is the first result, so `Enter` opens it. A part such as `DOC-001` lists the matching notes, and `doc` alone lists the newest. You can also type `/_id/DOC-0015` after the address (`http://127.0.0.1:8787/_id/DOC-0015`) to bookmark it, or run `docent id open DOC-0015` in a terminal. |
| Find what was written on a day | In `Ctrl K`, the **Created** row under the search box picks **Today**, **Yesterday** or any date. The list then shows the pages created that day, newest first, with their ID and time; typing narrows it. Days are those of your own time zone. |
| See when a note was created or changed | The **Created** and **Updated** chips under the title, and the overview tables, show the date and the time, in your browser's time zone. The front matter (`date:`, `updated:`) records only the day, so the time comes from the file itself, and only when the file's clock agrees with that day. A note that was copied or restored shows the day alone (hovering says so). |
| Keep the title in sight | Once the title scrolls away, the top bar shows the page title and the note ID in place of the folder path. Click the ID to copy it, or the title to go back to the top. |
| Move around | The left column is the folder tree. Links between pages work as you expect; broken links are marked. |
| Find a page in a long folder | Once a library has more than eight pages, a **Filter pages** box sits above the tree. Type words (they all have to match): `block users`, a folder name, a date, or a note ID. The tree shrinks to the matching pages with your words marked, and `Esc` clears it. |
| Read long titles in the tree | Pages whose titles start with the same words are shown from where they differ ("Payments block 5: users" shows as "Block 5: users"), long titles keep their last words visible ("Integration … decisions"), and hovering shows the full title and note ID. |
| Get to new notes | In a developer-notes library Docent remembers which notes you have not opened yet (in this browser; the first time you look at a library, everything already there counts as seen). **A toast** appears when a note is added while Docent is open (for example one Claude just wrote): *Open* takes you to it. **A dot** marks new notes in the page tree, and on a closed folder that holds one. **A count** ("2 new") shows on the library cards and in the library menu, and a dot on the library button when another library has news. **What's new** on the home page lists new notes and the ones from the last week, newest first, with *Mark all as seen*. Press `n` (or choose it in `Ctrl K`) to open the newest note you have not seen; with nothing typed, `Ctrl K` lists the new notes first. Opening a note marks it seen. |
| Wrap long lines of code | Long lines in a code block scroll sideways by default. Each block has a **wrap button** (next to *Copy*) that wraps its lines so the whole command is in view; copying still gives the original lines. To wrap every block, choose **Wrap** under *Long code lines* in the reading controls (the **Aa** button), or `Ctrl K` and "Wrap long code lines". Docent remembers it, and on paper (print and PDF) lines always wrap. |
| Read wide tables | A table that is wider than the text takes the **width of the page**, while the text keeps its comfortable width. The **contents list** ("On this page") stays visible; the **contents button** in the top bar hides or shows it, and a table gets that column too when it is hidden. Docent remembers your choice. Long names such as routes break at `/`, `.` and `_` instead of stretching a column, and the header row stays under the top bar as you scroll. If a table still does not fit, it scrolls sideways with its **first column pinned**, so you always know which row you are on (click it and use the arrow keys). Tables with three or more columns have an **Enlarge** button: a big view with every column, a pinned header and first column, and **Copy as Markdown**. |
| Browse like a file manager | Press `b` (or the folder-tree button in the top bar) for the **Browse** popup: columns for your libraries, then folders, then pages, with the full titles, note IDs and a preview of the page you have selected. `↑` `↓` move, `→` goes into a folder, `←` (or Backspace) goes back, `Enter` opens a page (on a folder it goes in; `Shift+Enter` opens the folder's own page), `Esc` closes. Typing searches the library you are in and lists matches with where they live; `Esc` clears the search first. It starts at the page you are reading, works from any page, and on a phone shows one column at a time with a back arrow. |
| Make the tree wider | Drag the right edge of the left column (or focus it and use the arrow keys, `Home` and `End`); double-click it to reset. The width is remembered. |
| Jump inside a page | "On this page" on the right follows your scrolling. Hover a heading to copy a link to it, or the whole section as Markdown. |
| See what links here | "Referenced by" at the bottom of a page, and Previous / Next to walk through a folder. |
| Change the look | The **Aa** button: text size, line width, light or dark (or follow your system), and your editor. The monitor button hides the sidebars (focus mode). |
| Copy or save | The toolbar copies a page as Markdown, downloads it, or prints it. |
| Print or save as PDF | The printer button asks which sections to include. "Related notes" style sections are left out unless you choose them; every page gets a header and page numbers. |
| Open a file you were sent | Drop it on the window (a temporary copy), or `docent open file.md` (the real file). |
| Look at a diagram | Click it to enlarge, or open it in its own window. Diagrams written as ` ```mermaid ` blocks are drawn for you, in both themes. |

Everything updates by itself: save a file in your editor and the page changes in the browser within a fraction of a second.

## Drop a file, or open one in a hurry

Drag one or more `.md` files from your file manager onto the Docent window and they open straight away. A browser never tells a web page where a dropped file lives on disk, so Docent
cannot open the original: it opens a **temporary copy**. The page says so; edits to the original are not shown (drop it again to refresh; the same file name replaces the copy), and all copies are deleted when Docent quits.
They appear on the home page as **Dropped files**, never in your remembered libraries. Only files can be dropped, not folders (use **Open folder or file** for those), up to 50 files and 5 MB each.

`docent open <file>` instead opens the real file and follows your edits.

## Open a page on your phone

Press the phone button in the top bar, then **Start sharing**. Docent shows a QR code for the page you are reading; scan it with your phone's camera and the same page opens there.

- It only works when the phone and the computer are on the same network (guest Wi-Fi often blocks it, and a firewall may need to allow the port shown, 8788 or the next free one).
- While sharing is on, anyone on that network with the address can **read** what is open in Docent. Nobody can change or add anything. It stops by itself after the time you chose (15 minutes, 1 hour or 4 hours), when you press **Stop sharing**, or when Docent quits.
- Docent prints nothing and listens on nothing beyond your own computer until you start sharing.

## What Docent understands

Plain Markdown, plus tables, task lists, footnotes, `> [!NOTE]` and `!!! note` call-outs, code with syntax colours (about 70 common languages), and Mermaid diagrams.
A page may start with a block of settings between `---` lines (`title:`, `tags:`, ...); none are required. Images are not supported yet: they appear as their description text. 

## Keep it open to your network all the time

The phone button above is for the occasional share. To leave Docent reachable from other devices for as long as it runs:

```bash
docent serve --lan
```

It prints addresses such as `http://192.168.1.20:8787` that other devices on your network can open. In this mode Docent is **read-only** (no folder browser, no adding folders) and anyone on the network can read
what is open, so use it only on a network you trust.

## Run it in the background

If you want Docent ready at all times on Linux, `packaging/docent.service` is a ready-made systemd user service (instructions are at the top of the file).
On Windows, put a shortcut to `docent.exe` in your Startup folder (press `Win+R`, type `shell:startup`).
Otherwise just run `docent` when you want it; it starts in about a tenth of a second.

## If something is not right

| Symptom | What to try |
| ------- | ----------- |
| "Docent is already running" | It is: the command opens the running copy. |
| A page is missing | Only `.md` files are shown. Hidden folders and `node_modules` are skipped. `docent check <folder>` lists problems and broken links. |
| Port 8787 is busy | Docent uses the next free port and prints it; or choose one with `--addr 127.0.0.1:9000`. |
| The page looks stale | Reload once; if a folder is on a network drive, changes may not be announced by the system (reload the page). |
| Diagrams show "could not be drawn" | The message names the problem in the diagram source. `docent check <library> --diagrams` finds every such diagram. |

Your settings and the list of libraries live in `~/.config/docent/` (on Windows `%AppData%\docent`); the search cache in `~/.cache/docent/`
(on Windows `%LocalAppData%\docent`). Delete them to start fresh.

## For teams that keep developer notes

Libraries added with `--profile devnotes` add note IDs, templates, commit tracking and more. Run `docent --help` and `docent <command> --help` to see them.

### Plans: where is this task, and what is next?

A `plan` note is one note per task, kept up to date as the work goes on. Docent draws it as a status board at the top of the note: how far along you are, the main stages (Design, Plan, Develop, Testing, Release, or whatever the task needs), the steps of the stage you are in, what is happening **Now**, what is **Next**, and anything you are **waiting on others** for, with how many days it has waited. The home page lists every plan in progress as one row each, so you can see all your open tasks at once.

- **Create one:** `docent new <service> plan "Title"` (or `platform` for work across services). The template has five default stages; add, remove or rename steps and stages for the task.
- **Read it:** click a stage to see its steps. A dashed amber outline means the stage is held up by something you are waiting for.
- **Update it:** tick a step in the page (the check circle marks it done, the small play button marks it as the one you are working on), or use the commands below. The first tick asks once whether Docent may edit the note; only the plan block of its front matter and the `updated` date are written.
- **Waiting on others:** the tick next to an item removes it once it is done.

```bash
docent plan show <note>                 # the board in the terminal (<note> is an ID like DOC-0012, a path or part of the file name)
docent plan list                        # every plan in progress
docent plan start|done|block|skip|reset <note> "Stage/Step" ...
docent plan add-stage|add-step|rm <note> ...
docent plan wait <note> "Kafka topic created" --who DevOps --blocks Release
docent plan unwait <note> 1
docent plan now|next <note> "text"      # optional lines; they clear themselves when a step changes
```

When every step is done the plan is marked `done` and leaves the home page. A plan nobody has touched for five days, or an item waited on for five days, gets a warning at the top of the note.
