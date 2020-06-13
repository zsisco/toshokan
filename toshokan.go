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
	Title string
	Authors string
	Year string
	Tags string
	Read bool
	BibText string
	Notes string
}

type EntryMap map[string]*Entry

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

// Default paths
const BIBS       = ".bibs/"
const LIBRARY    = "./library/"
const NOTES      = ".notes/"
const TOSHOKAN   = "./toshokan.json"

// Default apps
const EDITOR     = "vim"
const PDF_VIEWER = "mupdf"

// Default tags
const ALL_TAG    = "--ALL--"
const READ_TAG   = "--READ--"
const UNREAD_TAG = "--UNREAD--"

// Globals
var current_focus int
var toshokan EntryMap

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

func MakeFilename(author, year, title, ext string) string {
	encodeAuthor := strings.Replace(author, " ", "-", -1)
	encodeTitle := strings.Replace(title, " ", "-", -1)
	return encodeAuthor + "_" + year + "_" + encodeTitle + "." + ext
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
		if _, exists := toshokan[file]; !exists {
			// parse filename for authors_year_title
			fileparts := strings.Split(strings.Split(file, ".")[0], "_")
			authors := strings.Replace(fileparts[0], "-", " ", -1)
			year := fileparts[1]
			title := strings.Replace(fileparts[2], "-", " ", -1)
			toshokan[file] = &Entry{Title: title,
									Authors: authors,
									Year: year,
								    Read: false}
		}
	}

	for key := range toshokan {
		missingFile := true
		for _, file := range files {
			if key == file {
				missingFile = false
				break
			}
		}
		if missingFile {
			delete(toshokan, key)
		}
	}
}

func MakeTagSet(entries EntryMap) map[string]bool {
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
	// clear table
	for row := 0; row < table.GetRowCount(); row++ {
		table.RemoveRow(row)
	}
	titleToFile := make(map[string]string)
	for k := range toshokan {
		titleToFile[toshokan[k].Title] = k
	}
	// sort titles alphabetically
	var titles []string
	for k := range toshokan {
		titles = append(titles, toshokan[k].Title)
	}
	sort.Strings(titles)

	row := 0
	for _,title := range titles {
		filename := titleToFile[title]
		entryTags := MakeTagSet(EntryMap {filename: toshokan[filename]})
		if tag == ALL_TAG ||
			entryTags[tag] ||
			(tag == READ_TAG && toshokan[filename].Read) ||
			(tag == UNREAD_TAG && !toshokan[filename].Read) {
			table.SetCell(row, READ, CreateCell(BoolToReadFlag(toshokan[filename].Read), tview.AlignLeft, true))
			table.SetCell(row, TITLE, CreateCell(toshokan[filename].Title, tview.AlignLeft, true))
			table.SetCell(row, AUTHORS, CreateCell(toshokan[filename].Authors, tview.AlignLeft, true))
			table.SetCell(row, YEAR, CreateCell(toshokan[filename].Year, tview.AlignLeft, true))
			row++
		}
	}
	table.SetBorder(false)
	table.SetTitle(tag)
	table.SetSelectable(true, false)
	if current_focus == LIB_FOCUS {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDefault, 0)
	} else {
		table.SetSelectedStyle(tcell.ColorDefault, tcell.ColorDarkGray, 0)
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
}

func Check(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func main() {
	// Initialze!
	app := tview.NewApplication()

	// Library table
	table := tview.NewTable().SetFixed(1, 3)

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
	   [ ] t: edit tags
	   [X] q: toggle read flag
	   [ ] w: open notes file in text editor (create file in not exists); how to open vim inside the program like aerc?
	   [ ] e: open bib file in text editor (create file in not exists)
	   [ ] /: search meta data in current view (moves cursor with n/N search results) [ADVANCED]
	   [X] tab: swap focus between library table and tags table
	 */

		switch event.Key() {
		case tcell.KeyEscape:
			if freeInput { return event }
			app.Stop()
			return nil
		case tcell.KeyTab:
			if freeInput { return event }
			if current_focus == LIB_FOCUS {
				current_focus = TAG_FOCUS
				app.SetFocus(tagsView)
			} else {
				current_focus = LIB_FOCUS
				app.SetFocus(table)
			}
			RedrawScreen(table, tagsView)
		case tcell.KeyEnter:
			if freeInput { return event }
			if current_focus == TAG_FOCUS {
				RedrawScreen(table, tagsView)
			}
			if current_focus == LIB_FOCUS {
				row, _ := table.GetSelection()
				selectedFile := MakeFilename(table.GetCell(row, AUTHORS).Text,
											 table.GetCell(row, YEAR).Text,
											 table.GetCell(row, TITLE).Text,
											 "pdf")
				cmd := exec.Command(PDF_VIEWER, LIBRARY + selectedFile)
				err := cmd.Start()
				Check("Error launching PDF viewer", err)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				// refresh table view
				if freeInput { return event }
				Refresh(table, tagsView)
				return nil
			case 't':
				// edit meta data
				if freeInput { return event }
				if current_focus == TAG_FOCUS { return event }
				freeInput = true
				row, _ := table.GetSelection()
				filename := MakeFilename(table.GetCell(row, AUTHORS).Text,
											 table.GetCell(row, YEAR).Text,
											 table.GetCell(row, TITLE).Text,
											 "pdf")
				newTags := ""
				// TODO: change input field colors to something always readable
				metadataForm := tview.NewForm().
					AddInputField("Tags (semicolon-separated)", toshokan[filename].Tags, 0, nil, func(changed string) {
						newTags = changed
					}).
					AddButton("Save", func() {
						if newTags != "" {
							toshokan[filename].Tags = newTags
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
				metadataForm.SetLabelColor(tcell.ColorDefault)
				metadataForm.SetFieldBackgroundColor(tcell.ColorDefault)
				metadataForm.SetButtonBackgroundColor(tcell.ColorDarkGray)
				metadataForm.SetBorder(true).
					SetTitle("Metadata: " + table.GetCell(row, TITLE).Text).
					SetTitleAlign(tview.AlignLeft)
				pages.AddAndSwitchToPage("metadata", metadataForm, true)
				return nil
			case 'q':
				// toggle read/unread
				if freeInput { return event }
				if current_focus == TAG_FOCUS { return event }
				row, _ := table.GetSelection()
				filename := MakeFilename(table.GetCell(row, AUTHORS).Text,
											 table.GetCell(row, YEAR).Text,
											 table.GetCell(row, TITLE).Text,
											 "pdf")
				newReadFlag := SwapReadFlag(table.GetCell(row, READ).Text)
				table.SetCell(row, READ, CreateCell(newReadFlag, tview.AlignLeft, true))
				toshokan[filename].Read = ReadFlagToBool(newReadFlag)
				WriteToJson()
				return nil
			}
			case 'w':
				if freeInput { return event }
				if current_focus == TAG_FOCUS { return event }
				// open notes in EDITOR
				// XXX: this isn't working, may need to steal from aerc
				row, _ := table.GetSelection()
				filename := MakeFilename(table.GetCell(row, AUTHORS).Text,
											 table.GetCell(row, YEAR).Text,
											 table.GetCell(row, TITLE).Text,
											 "md")
				editor := exec.Command("/bin/sh -c", EDITOR +" .notes/" + filename)
				editor.Stdin = os.Stdin
				editor.Stdout = os.Stdout
				editor.Stderr = os.Stderr
				ederror := editor.Run()
				Check("error opening external editor", ederror)
				return nil
			case 'e':
				if freeInput { return event }
				if current_focus == TAG_FOCUS { return event }
				// open bibtex in EDITOR
				return nil
			case '/':
				if freeInput { return event }
				if current_focus == TAG_FOCUS { return event }
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
