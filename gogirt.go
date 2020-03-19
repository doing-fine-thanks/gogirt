package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/fatih/color"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

const version = "0.0.1 - Dont use this."
const usage = `
Manage micro repos poorly.

Usage:
  gogirt status <profile>
  gogirt broadcast <profile> <command> 
  gogirt -h | --help
  gogirt --version

Options:
  -h --help     Show this screen.
  --version     Show version.
  --filter=<dirs_to_filter_to>...  A list of directory names to filter to.
`


// Make my GitRoots pretty, of please lord.
const PrettyPrintStringForGitRoot = `
Project %s Overview (Branch %s):
 - Project state is clean: %s
`
// Don't ruin my computer with endless searches.
const maxDepth = 3


// COLORS!!!
var yellow = color.New(color.FgYellow).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()
var cyan = color.New(color.FgCyan).SprintFunc()

// A deeply small struct to represent paths to git projects.
type GitRoot struct {
	Path    string
}

// Serialize a GitRoot into JSON.
func (t GitRoot) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// Get the status message for a GitRoot.
func (t GitRoot) getStatus() string {
	return issueCommand("git status", t.Path, true)
}

// Is the Git Project clean?
func (t GitRoot) isClean() bool {
	return strings.Contains(t.getStatus(), "working tree clean")
}

// Get that GitRoots branch.
func (t GitRoot) getBranch() string {
	return issueCommand( "git rev-parse --abbrev-ref HEAD", t.Path, true)
}

// Make the GitRoot pretty.
func (t GitRoot) PrettyPrint() {
	var cleanText string
	rootIsClean := t.isClean()
	if !rootIsClean{
		cleanText = red(rootIsClean)
	} else {
		cleanText = yellow(rootIsClean)
	}

	fmt.Printf(PrettyPrintStringForGitRoot, cyan(t.Path), yellow(t.getBranch()), cleanText)

	if !rootIsClean {
		fmt.Printf(red(t.getStatus()))
	}
}


// Construct an array of GitRoots from a list of paths.
func makeGitRootsFromPath(paths []string) []GitRoot {
	var roots []GitRoot
	for _, path := range paths {
		roots = append(roots, GitRoot{Path:path[:len(path) - 5]})
	}
	return roots
}

// Issue a command on some path. If the output is not set to return, it
// will be redirected to stdout.
func issueCommand(command string, path string, returnOutput bool) string {
	commandList := strings.Split(command, " ")
	cmd := exec.Command(commandList[0], commandList[1:len(commandList)]...)
	cmd.Dir = path

	if returnOutput {
		status, err := cmd.Output()
		if err != nil {
			panic(err)
		}
		return strings.TrimSpace(string(status))
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
		return ""
	}
}

// Check to see if a path is in a list of filter paths.
func isFilterDirInGitPath(path string, filter string) bool {
	splitPath := strings.Split(path, "/")
	terminalDirectory := splitPath[len(splitPath) - 2]
	for _, filterPath := range strings.Split(filter, ",") {
		if filterPath == terminalDirectory{
			return true
		}
	}

	return false
}

// Get paths from a root path out to some max depth that contain a .git
// and (if non-empty) are contained in a filter.
func getGitPaths(rootDir string, filter string) []string {
	var filePaths []string
	err := filepath.Walk(rootDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			} else if strings.Count(path, "/") >= (maxDepth + strings.Count(rootDir, "/")) { // if we go too deep
				return filepath.SkipDir
			} else if info.IsDir() && len(path) > 5 && path[len(path)-5:] == "/.git" {
				if len(filter) == 0 || isFilterDirInGitPath(path, filter) {  // if we aren't filtering or the path is in the filter
					filePaths = append(filePaths, path)
					return filepath.SkipDir
				}
			}

			return nil
		})

	if err != nil {
		log.Fatal(err)
	}

	return filePaths
}

// Get the users response to what they want to do with a dirty git repo.
func getDirtyHandleChoice(root GitRoot) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(red("Directory %s is in a dirty state!!!\n"), root.Path)
	fmt.Println("Do you want to continue anyway (c), manually resolve it (m), hard reset prior (h), or skip (s)?")
	fmt.Print("answer (c/m/h/s): ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == ""{
		text = "n o t h i n g"
	}

	return text
}

type Profile struct {
	Name string    `json:"name"`
	Rootdir string `json:"rootdir"`
	Filter string  `json:"filter"`
}

type Config struct {
	ShellCommand string `json:"shellCommand"`
	Profiles []Profile `json:"profiles"`
}

// Serialize a GitRoot into JSON.
func (t Config) JSON() []byte {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	if err != nil {
		panic(err)
	}

	return buffer.Bytes()
}


func getConfig() Config {
	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}
	confPath := currentUser.HomeDir + "/.gogirtConf"

	jsonFile, err := ioutil.ReadFile(confPath)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	err = json.Unmarshal(jsonFile, &config)

	return config
}

func getProfile(config Config, name string) (Profile, error) {
	for _, profile := range config.Profiles {
		if profile.Name == name {
			return profile, nil
		}
	}

	return Profile{}, os.ErrNotExist
}

func main() {
	opt, _ := docopt.ParseDoc(usage)
	var args struct {
		Status bool
		Broadcast bool
		Rootdir string
		Filter string
		Command string
		Version bool
		Profile string
	}
	_ = opt.Bind(&args)

	if args.Version {
		println(version)
		println(usage)
		return
	}

	config := getConfig()
	profile, err := getProfile(config, args.Profile)
	if err != nil {
		log.Fatal("No profile with name ", args.Profile)
	}

	filePaths := getGitPaths(profile.Rootdir, profile.Filter)
	roots := makeGitRootsFromPath(filePaths)

	for _, root := range roots{
		if args.Status {
			root.PrettyPrint()
		} else if args.Broadcast{
			if !root.isClean() {
				text := getDirtyHandleChoice(root)
				if text == "m" {
					fmt.Println("Manually resolving (press ctl-D to quit, or type 'exit')...")
					issueCommand(config.ShellCommand, root.Path, false)
				} else if text == "h" {
					fmt.Println("Hard Resetting, then Broadcasting...")
					issueCommand("git reset --hard", root.Path, false)
				} else if text == "s" {
					fmt.Print("Skipping...\n\n\n")
					continue
				} else if text != "c" {
					log.Fatal(fmt.Sprintf("No clue what to do with %s, just skipping...", text))
				}
			}
			fmt.Printf("Broadcasting %s to directory %s\n\n", yellow(args.Command), yellow(root.Path))
			issueCommand(args.Command, root.Path, false)
			println("\n\n")
		}
	}

	time.Sleep(500 * time.Millisecond)
}
