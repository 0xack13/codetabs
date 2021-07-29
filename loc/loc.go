/* */

package loc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	u "github.com/jolav/codetabs/_utils"
)

const (
	SCC      = "./_data/loc/scc"
	MAX_SIZE = 500
)

type loc struct {
	order        string
	orderInt     int
	repo         string
	source       string
	date         string
	size         int
	sr           sourceReader
	languagesIN  []languageIN
	languagesOUT []languageOUT
}

type languageIN struct {
	Name     string `json:"Name"`
	Files    int    `json:"count"`
	Lines    int    `json:"lines"`
	Blanks   int    `json:"blank"`
	Comments int    `json:"comment"`
	Code     int    `json:"code"`
}

type languageOUT struct {
	Name     string `json:"language"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
	Blanks   int    `json:"blanks"`
	Comments int    `json:"comments"`
	Code     int    `json:"linesOfCode"`
}

type sourceReader interface {
	existRepo(string) bool
	exceedsSize(http.ResponseWriter, *loc) bool
}

func (l *loc) Router(w http.ResponseWriter, r *http.Request) {
	params := strings.Split(strings.ToLower(r.URL.Path), "/")
	path := params[1:len(params)]
	if path[len(path)-1] == "" { // remove last empty slot after /
		path = path[:len(path)-1]
	}
	//log.Printf("Going ....%s %s %d", path, r.Method, len(path))
	if len(path) < 2 || path[0] != "v1" {
		u.BadRequest(w, r)
		return
	}
	// clean
	l = &loc{
		repo:         "",
		source:       "",
		date:         "",
		size:         0,
		languagesIN:  []languageIN{},
		languagesOUT: []languageOUT{},
	}

	if r.Method == "POST" {
		l.orderInt++
		l.order = strconv.Itoa(l.orderInt)
		l.doLocUploadRequest(w, r)
		return
	}
	r.ParseForm()
	for k, _ := range r.URL.Query() {
		l.source = k
		l.repo = r.URL.Query()[k][0]
	}
	aux := strings.Split(l.repo, "/")
	if len(aux) != 2 || aux[0] == "" || aux[1] == "" {
		msg := fmt.Sprintf("Incorrect user/repo")
		u.ErrorResponse(w, msg)
		return
	}
	if len(path) != 2 {
		u.BadRequest(w, r)
		return
	}
	switch l.source {
	case "github":
		l.sr = github{}
	case "gitlab":
		l.sr = gitlab{}
	}
	l.orderInt++
	l.order = strconv.Itoa(l.orderInt)
	l.doLocRepoRequest(w, r)
}

func (l *loc) doLocRepoRequest(w http.ResponseWriter, r *http.Request) {

	// MOCK
	/*_ = json.Unmarshal([]byte(data), &l.languagesIN)
	total := languageOUT{
		Name: "Total",
	}
	for _, v := range l.languagesIN {
		l.languagesOUT = append(l.languagesOUT, languageOUT(v))
		total.Blanks += v.Blanks
		total.Code += v.Code
		total.Comments += v.Comments
		total.Files += v.Files
		total.Lines += v.Lines
	}
	l.languagesOUT = append(l.languagesOUT, total)
	u.SendJSONToClient(w, l.languagesOUT, 200)
	return*/
	//

	if !l.sr.existRepo(l.repo) {
		msg := l.repo + " doesn't exist"
		u.ErrorResponse(w, msg)
		return
	}
	if l.sr.exceedsSize(w, l) {
		msg := fmt.Sprintf(`repo %s too big (>%dMB) = %d MB`,
			l.repo,
			MAX_SIZE,
			l.size,
		)
		u.ErrorResponse(w, msg)
		return
	}

	folder := "_tmp/loc/" + l.order
	destroyTemporalDir := []string{"rm", "-rf", folder}
	createTemporalDir := []string{"mkdir", folder}

	err := u.GenericCommand(createTemporalDir)
	if err != nil {
		log.Printf("ERROR cant create temporal dir %s\n", err)
		msg := "Cant create temporal dir for " + l.repo
		u.ErrorResponse(w, msg)
		return
	}

	url := "https://" + l.source + ".com/" + l.repo
	dest := "./" + folder
	cloneRepo := []string{"git", "clone", url, dest}
	err = u.GenericCommand(cloneRepo)
	if err != nil {
		log.Printf("ERROR Cant clone repo %s -> %s\n", err, r.URL.RequestURI())
		msg := "Can't clone repo " + l.repo
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}
	repoPath := "./" + folder
	info, err := l.countLines(repoPath)
	if err != nil {
		log.Printf("ERROR counting loc %s -> %s\n", err, r.URL.RequestURI())
		msg := "Error counting LOC in " + l.repo
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}

	err = json.Unmarshal(info, &l.languagesIN)
	if err != nil {
		log.Printf("ERROR unmarshal LOC %s\n", err)
	}

	total := languageOUT{
		Name: "Total",
	}
	for _, v := range l.languagesIN {
		l.languagesOUT = append(l.languagesOUT, languageOUT(v))
		total.Blanks += v.Blanks
		total.Code += v.Code
		total.Comments += v.Comments
		total.Files += v.Files
		total.Lines += v.Lines
	}
	l.languagesOUT = append(l.languagesOUT, total)

	u.SendJSONToClient(w, l.languagesOUT, 200)
	u.GenericCommand(destroyTemporalDir)
}

func (l *loc) countLines(repoPath string) (info []byte, err error) {
	comm := SCC + " " + repoPath + " -f json "
	fmt.Println("COMMAND => ", comm)
	info, err = u.GenericCommandSH(comm)
	if err != nil {
		log.Println(fmt.Sprintf("ERROR in countLines %s\n", err))
		return nil, err
	}
	return info, err
}

func (l *loc) doLocUploadRequest(w http.ResponseWriter, r *http.Request) {
	folder := "_tmp/loc/" + l.order
	destroyTemporalDir := []string{"rm", "-rf", folder}
	createTemporalDir := []string{"mkdir", folder}
	err := u.GenericCommand(createTemporalDir)
	if err != nil {
		log.Printf("ERROR 1 creating folder %s\n", err)
		msg := "Error creating folder " + folder
		u.ErrorResponse(w, msg)
		return
	}

	// create file
	file, handler, err := r.FormFile("inputFile")
	if err != nil {
		log.Printf("ERROR creating file %s\n", err)
		msg := "Error creating file "
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}
	upload := handler.Filename
	filePath := "./" + folder + "/" + upload
	defer file.Close()
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("ERROR opening uploaded file %s\n", err)
		msg := "Error opening " + upload
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	dest := "./" + folder
	//unzipFile := []string{"unzip", filePath, "-d", dest + "/src"}
	unzipFile := []string{"7z", "x", filePath, "-o" + dest + "/src"}
	err = u.GenericCommand(unzipFile)
	if err != nil {
		log.Printf("ERROR 7z %s -> %s\n", err, r.URL.RequestURI())
		msg := "Error unziping " + upload
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}

	repoPath := dest + "/src"
	info, err := l.countLines(repoPath)
	if err != nil {
		log.Printf("ERROR counting loc %s -> %s\n", err, r.URL.RequestURI())
		msg := "Error counting LOC in " + upload
		u.ErrorResponse(w, msg)
		u.GenericCommand(destroyTemporalDir)
		return
	}

	err = json.Unmarshal(info, &l.languagesIN)
	if err != nil {
		log.Printf("ERROR unmarshal LOC %s\n", err)
	}

	total := languageOUT{
		Name: "Total",
	}
	for _, v := range l.languagesIN {
		l.languagesOUT = append(l.languagesOUT, languageOUT(v))
		total.Blanks += v.Blanks
		total.Code += v.Code
		total.Comments += v.Comments
		total.Files += v.Files
		total.Lines += v.Lines
	}
	l.languagesOUT = append(l.languagesOUT, total)

	u.SendJSONToClient(w, l.languagesOUT, 200)
	u.GenericCommand(destroyTemporalDir)
}

func NewLoc(test bool) loc {
	l := loc{
		order:        "0",
		orderInt:     0,
		repo:         "",
		source:       "",
		date:         "",
		size:         0,
		languagesIN:  []languageIN{},
		languagesOUT: []languageOUT{},
	}
	return l
}
