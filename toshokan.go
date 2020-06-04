package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type Entry struct {
	Filename string
	Title string
	Authors string
	Year string
	Tags []string
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

const BIBS       = ".bibs/"
const EDITOR     = "vim"
const LIBRARY    = "./library/"
const NOTES      = ".notes/"
const PDF_VIEWER = "mupdf"
const TOSHOKAN   = "./toshokan.json"

func main() {
	BoolToReadFlag := func(b bool) string {
		if b {
			return "o"
		} else {
			return "-"
		}
	}

	ReadFlagToBool := func(f string) bool {
		if f == "o" {
			return true
		} else {
			return false
		}
	}

	SwapReadFlag := func(f string) string {
		if f == "o" {
			return "-"
		} else {
			return "o"
		}
	}

	CreateCell := func(content string, align int, selectable bool) *tview.TableCell {
		return tview.NewTableCell(content).
			SetAlign(align).
			SetSelectable(selectable)
	}

	WriteToJson := func(toshokan *[]Entry) {
		bytes, merr := json.Marshal(toshokan)
		Check("json.Marshal error", merr)
		werr := ioutil.WriteFile(TOSHOKAN, bytes, 0644)
		Check("WriteFile error", werr)
	}

	ReadFromJson := func(toshokan *[]Entry) {
		toshokanFile, readerr := ioutil.ReadFile(TOSHOKAN)
		Check("ReadFile error", readerr)
		uerr := json.Unmarshal(toshokanFile, &toshokan)
		Check("json.Unmarshal error", uerr)
	}

	// TODO:
	// this should also refresh the library directory 
	// and adding any entries
	ScanLibrary := func(toshokan []Entry) []Entry {
		// scan LIBRARY directory and add an Entry to toshokan for every
		// filename not there. If a file was removed from the dir but is
		// still in the JSON/toshokan, remove it.
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
		for i, entry := range toshokan {
			missingFile := true
			for _, file := range files {
				if entry.Filename == file {
					missingFile = false
					break
				}
			}
			if missingFile {
				// remove entry from toshokan
				toshokan[len(toshokan) - 1], toshokan[i] = toshokan[i], toshokan[len(toshokan) - 1]
				toshokan = toshokan[:len(toshokan) - 1]
			}
		}
		return toshokan
	}

	RedrawTable := func(table *tview.Table, toshokan []Entry) {
		for row, entry := range toshokan {
			// TODO: check tags with current tag selection (plus exceptions like "All", "Read"/"Unread")
			table.SetCell(row, READ, CreateCell(BoolToReadFlag(entry.Read), tview.AlignLeft, true))
			table.SetCell(row, TITLE, CreateCell(entry.Title, tview.AlignLeft, true))
			table.SetCell(row, AUTHORS, CreateCell(entry.Authors, tview.AlignLeft, true))
			table.SetCell(row, YEAR, CreateCell(entry.Year, tview.AlignLeft, true))
		}
		table.SetBorder(true)
		table.SetTitle("All") // TODO: update with tag selection
		table.SetSelectable(true, false)
	}

	Refresh := func(table *tview.Table, toshokan *[]Entry) {
		ReadFromJson(toshokan)
		*toshokan = ScanLibrary(*toshokan)
		RedrawTable(table, *toshokan)
		//WriteToJson(toshokan)
	}

	// TODO: create left frame for tags
	// navigate by J/K
	// selection updates the table title, refreshes the table view with
	// only entries that match the tag
	// TODO: built-in tag selections for "All" and "Read"/"Unread" to be able
	// to filter by read/unread.


	// Initialze!
	var toshokan []Entry
	
	app := tview.NewApplication()

	table := tview.NewTable().SetFixed(1, 1)

	pages := tview.NewPages()
	pages.AddPage("library", table, true, true)

	Refresh(table, &toshokan)

	app.SetFocus(table)

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
	   [ ] J: tag up
	   [ ] K: tag down
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
				Refresh(table, &toshokan)
				return nil
			case 'e':
				// edit meta data
				if freeInput { return event }
				freeInput = true
				row, _ := table.GetSelection()
				newTitle := ""
				newAuthors := ""
				newYear := ""
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
					// TODO: tags field
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
						freeInput = false
						WriteToJson(&toshokan)
						RedrawTable(table, toshokan)
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
				WriteToJson(&toshokan)
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
				ed_error := editor.Run()
				Check("error opening external editor", ed_error)
				return nil
			case 't':
				if freeInput { return event }
				// open bibtex in EDITOR
				return nil
			case '/':
				if freeInput { return event }
				// search
				return nil
			case 'J':
				// tag list down
				if freeInput { return event }
				return nil
			case 'K':
				// tag list up
				if freeInput { return event }
				return nil
			// Fall through to capture table-level input events like j,k,h,l,...
			return event
		}
		return event
	})

	rooterr := app.SetRoot(pages, true).Run()
	Check("SetRoot error", rooterr)
}

func Check(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}
