package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type Entry struct {
	Filename string
	Title string
	Authors string
	Year string
	Tags string
	Read bool
	BibText string
	Notes string
}

const (
	READ = iota
	TITLE
	AUTHORS
	YEAR
)

const (
	LIB_FOCUS = iota
	TAG_FOCUS
)

const BIBS       = ".bibs/"
const EDITOR     = "vim"
const LIBRARY    = "./library/"
const NOTES      = ".notes/"
const PDF_VIEWER = "mupdf"
const TOSHOKAN   = "./toshokan.json"

// Default tags
const ALL_TAG    = "--ALL--"
const READ_TAG   = "--READ--"
const UNREAD_TAG = "--UNREAD--"

// Globals
var current_focus int
var toshokan []Entry

func BoolToReadFlag(b bool) string {
	if b {
		return "o"
	} else {
		return "-"
	}
}

func ReadFlagToBool(f string) bool {
	if f == "o" {
		return true
	} else {
		return false
	}
}

func SwapReadFlag(f string) string {
	if f == "o" {
		return "-"
	} else {
		return "o"
	}
}

func CreateCell(content string, align int, selectable bool) *tview.TableCell {
	return tview.NewTableCell(content).
		SetAlign(align).
		SetSelectable(selectable)
}

func WriteToJson() {
	bytes, merr := json.Marshal(toshokan)
	Check("json.Marshal error", merr)
	werr := ioutil.WriteFile(TOSHOKAN, bytes, 0644)
	Check("WriteFile error", werr)
}

func ReadFromJson() {
	toshokanFile, readerr := ioutil.ReadFile(TOSHOKAN)
	Check("ReadFile error", readerr)
	uerr := json.Unmarshal(toshokanFile, &toshokan)
	Check("json.Unmarshal error", uerr)
}

func ScanLibrary() {
	// scan LIBRARY directory and add an Entry to toshokan for every
	// filename not there. If a file was removed from the dir but is
	// still in the JSON/toshokan, remove its entry.
	// Run this _after_ ReadFromJson is called.
	// O(2 * n^2)...
	var files []string

	err := filepath.Walk(LIBRARY, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		files = append(files, info.Name())
		return nil
	})
	Check("ReadDir error!", err)
	for _, file := range files {
	//	fmt.Println(file)
		noEntry := true
		for _, entry := range toshokan {
			if entry.Filename == file {
				noEntry = false
				break
			}
		}
		if noEntry {
			// add to toshokan
			newEntry := Entry{Filename: file,
							  Title: file,
							  Read: false}
			toshokan = append(toshokan, newEntry)
		}
	}

	for i := len(toshokan) - 1; i >= 0; i-- {
		missingFile := true
		for _, file := range files {
			if toshokan[i].Filename == file {
				missingFile = false
				break
			}
		}
		if missingFile {
			// remove entry from toshokan
			toshokan = append(toshokan[:i], toshokan[i + 1:]...)
		}
	}
}

func MakeTagSet(entries []Entry) map[string]bool {
	tagSet := make(map[string]bool)
	for _, entry := range entries {
		splitTags := strings.Split(entry.Tags, ";")
		for _, t := range splitTags {
			trimmedTag := strings.TrimSpace(t)
			if trimmedTag != "" {
				tagSet[trimmedTag] = true
			}
		}
	}
	return tagSet
}

func RedrawTable(table *tview.Table, tag string) {
	for row, entry := range toshokan {
		// TODO: check tags with current tag selection (plus exceptions like "All", "Read"/"Unread")
		entryTags := MakeTagSet([]Entry{entry})
		if tag == ALL_TAG ||
			entryTags[tag] ||
			(tag == READ_TAG && entry.Read) ||
			(tag == UNREAD_TAG && !entry.Read) {
			table.SetCell(row, READ, CreateCell(BoolToReadFlag(entry.Read), tview.AlignLeft, true))
			table.SetCell(row, TITLE, CreateCell(entry.Title, tview.AlignLeft, true))
			table.SetCell(row, AUTHORS, CreateCell(entry.Authors, tview.AlignLeft, true))
			table.SetCell(row, YEAR, CreateCell(entry.Year, tview.AlignLeft, true))
		}
	}
	table.SetBorder(false)
	table.SetTitle(tag)
	table.SetSelectable(true, false)
	if current_focus == LIB_FOCUS {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDefault, 0)
	} else {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorGray, 0)
	}
}

func RedrawTags(table *tview.Table) {
	tagSet := MakeTagSet(toshokan)
	// sort tags alphabetically
	var tags []string
	// Add default tags
	tags = append(tags, ALL_TAG)
	tags = append(tags, READ_TAG)
	tags = append(tags, UNREAD_TAG)
	for k := range tagSet {
		tags = append(tags, k)
	}
	sort.Strings(tags)

	for i, k := range tags {
		table.SetCell(i, 0, CreateCell(k, tview.AlignLeft, true))
	}
	table.SetBorder(false)
	table.SetSelectable(true, false)
	if current_focus == TAG_FOCUS {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDefault, 0)
	} else {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDarkGray, 0)
	}
}

func RedrawScreen(table *tview.Table, tags *tview.Table) {
	RedrawTags(tags)
	selectedTag := tags.GetCell(tags.GetSelection()).Text
	RedrawTable(table, selectedTag)
}

func Refresh(table *tview.Table, tags *tview.Table) {
	ReadFromJson()
	ScanLibrary()
	RedrawScreen(table, tags)
	//WriteToJson() // XXX: uncomment, or leave outside of here
}

func Check(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func main() {

	// selection updates the table title, refreshes the table view with
	// only entries that match the tag
	// TODO: built-in tag selections for "All" and "Read"/"Unread" to be able
	// to filter by read/unread.

	// Initialze!
	app := tview.NewApplication()

	// Library table
	table := tview.NewTable().SetFixed(1, 2)

	// Tags table
	tagsView := tview.NewTable()

	// Main page view (swaps out library, metadata editor, text editor)
	pages := tview.NewPages()
	pages.AddPage("library", table, true, true)

	layout := tview.NewFlex().
			  AddItem(tagsView, 0, 1, false).
			  AddItem(pages, 0, 4, true)

	app.SetFocus(table)
	current_focus = LIB_FOCUS

	Refresh(table, tagsView)

	freeInput := false
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
	/* TODO: 
	   [X] enter: open in pdf viewer
	   [X] r: refresh table view (reload json file) (also tags frame)
	   [X] e: edit meta data (title, authors, year, tags, filename (readonly))
	   [X] q: toggle read flag
	   [ ] w: open notes file in text editor (create file in not exists); how to open vim inside the program like aerc?
	   [ ] t: open bib file in text editor (create file in not exists)
	   [ ] /: search meta data in current view (moves cursor with n/N search results) [ADVANCED]
	   [ ] tab: swap focus between library table and tags table
	 */

		switch event.Key() {
		case tcell.KeyEscape:
			if freeInput { return event }
			app.Stop()
			return nil
		case tcell.KeyEnter:
			if freeInput { return event }
			row, _ := table.GetSelection()
			selectedFile := toshokan[row].Filename
			cmd := exec.Command(PDF_VIEWER, LIBRARY + selectedFile)
			err := cmd.Start()
			if err != nil {
				panic(err.Error())
				return nil
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				// refresh table view
				if freeInput { return event }
				Refresh(table, tagsView)
				return nil
			case 'e':
				// edit meta data
				if freeInput { return event }
				freeInput = true
				row, _ := table.GetSelection()
				newTitle := ""
				newAuthors := ""
				newYear := ""
				newTags := ""
				metadataForm := tview.NewForm().
					AddInputField("Title", toshokan[row].Title, 0, nil, func(changed string) {
						newTitle = changed
					}).
					AddInputField("Authors", toshokan[row].Authors, 0, nil, func(changed string) {
						newAuthors = changed
					}).
					AddInputField("Year", toshokan[row].Year, 4, nil, func(changed string) {
						newYear = changed
					}).
					AddInputField("Tags (semicolon-separated)", toshokan[row].Tags, 0, nil, func(changed string) {
						newTags = changed
					}).
					AddButton("Save", func() {
						if newTitle != "" {
							toshokan[row].Title = newTitle
						}
						if newAuthors != "" {
							toshokan[row].Authors = newAuthors
						}
						if newYear != "" {
							toshokan[row].Year = newYear
						}
						if newTags != "" {
							toshokan[row].Tags = newTags
						}
						freeInput = false
						WriteToJson()
						RedrawScreen(table, tagsView)
						pages.RemovePage("metadata")
					}).
					AddButton("Cancel", func() {
						freeInput = false
						pages.RemovePage("metadata")
					})
				metadataForm.SetBorder(true).
					SetTitle("File: " + toshokan[row].Filename).
					SetTitleAlign(tview.AlignLeft)
				pages.AddAndSwitchToPage("metadata", metadataForm, true)
				return nil
			case 'q':
				// toggle read/unread
				if freeInput { return event }
				row, _ := table.GetSelection()
				newReadFlag := SwapReadFlag(table.GetCell(row, READ).Text)
				table.SetCell(row, READ, CreateCell(newReadFlag, tview.AlignLeft, true))
				toshokan[row].Read = ReadFlagToBool(newReadFlag)
				WriteToJson()
				return nil
			}
			case 'w':
				if freeInput { return event }
				// open notes in EDITOR
				// XXX: this isn't working, may need to steal from aerc
				row, _ := table.GetSelection()
				editor := exec.Command("/bin/sh -c", EDITOR +" .notes/" + toshokan[row].Filename)
				editor.Stdin = os.Stdin
				editor.Stdout = os.Stdout
				editor.Stderr = os.Stderr
				ederror := editor.Run()
				Check("error opening external editor", ederror)
				return nil
			case 't':
				if freeInput { return event }
				// open bibtex in EDITOR
				return nil
			case '/':
				if freeInput { return event }
				// search
				return nil
			// Fall through to capture table-level input events like j,k,h,l,...
			return event
		}
		return event
	})

	rooterr := app.SetRoot(layout, true).Run()
	Check("SetRoot error", rooterr)
}
