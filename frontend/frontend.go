package frontend

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/user"
)

type Greeting struct {
	Author  string
	Content string
	Date    time.Time
}

func init() {
	http.HandleFunc("/", root)
	http.HandleFunc("/sign", sign)
	http.HandleFunc("/root", handleRoot)
	http.HandleFunc("/upload", handleUpload)
}

func serveError(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "Internal Server Error")
	c.Errorf("%v", err)
}

func guestbookKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "Guestbook", "default_guestbook", 0, nil)
}

func root(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html; charset=utf-8")
	c := appengine.NewContext(r)
	q := datastore.NewQuery("Greeting").Ancestor(guestbookKey(c)).Order("-Date").Limit(10)
	greetings := make([]Greeting, 0, 10)
	if _, err := q.GetAll(c, &greetings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := guestbookTemplate.Execute(w, greetings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// u := user.Current(c)
	// if u == nil {
	// 	fmt.Fprint(w, "This shouldn't be happening...")
	// 	return
	// }

	// if strings.HasSuffix(u.Email, "@google.com") {
	// 	fmt.Fprint(w, guestbookForm)
	// } else {
	// 	fmt.Fprint(w, "The hell are you???")
	// }

	// url, _ := user.LogoutURL(c, "/")
	// fmt.Fprintf(w, `(<a href="%s">sign out</a>)`, url)

}

var guestbookTemplate = template.Must(template.New("book").Parse(guestbookTemplateHTML))

const guestbookTemplateHTML = `
<html>
  <body>
    {{range .}}
      {{with .Author}}
        <p><b>{{.}}</b> wrote:</p>
      {{else}}
        <p>An anonymous person wrote:</p>
      {{end}}
      <pre>{{.Content}}</pre>
    {{end}}
    <form action="/sign" method="post">
      <div><textarea name="content" rows="3" cols="60"></textarea></div>
      <div><input type="submit" value="Sign Guestbook"></div>
    </form>
  </body>
</html>
`

var rootTemplate = template.Must(template.New("root").Parse(rootTemplateHTML))

const rootTemplateHTML = `
<html><body>
{{with .Files}}
<h1>Your files</h1>
{{range .}}
<p>{{index . 0}}({{index . 1}})</p>
{{end}}
{{end}}
<form action="{{.UploadUrl}}" method="POST" enctype="multipart/form-data">
Upload File: <input type="file" name="file"><br>
<input type="submit" name="submit" value="Submit">
</form></body></html>
`

func handleRoot(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)

	uploadURL, err := blobstore.UploadURL(c, "/upload", &blobstore.UploadURLOptions{StorageBucket: fmt.Sprintf("my_bucket/%v", u.ID)})
	if err != nil {
		serveError(c, w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	q := datastore.NewQuery("__GsFileInfo__").KeysOnly().
		Filter("filename >", fmt.Sprintf("/my_bucket/%v/", u.ID)).
		Filter("filename <", fmt.Sprintf("/my_bucket/%v0/", u.ID)).
		Order("filename")
	it := q.Run(c)
	files := [][]string{}
	for {
		cosa, err := it.Next(nil)
		if err == datastore.Done {
			break
		} else if err != nil {
			serveError(c, w, err)
			break
		}

		info, err := blobstore.Stat(c, appengine.BlobKey(cosa.StringID()))
		if err != nil {
			serveError(c, w, err)
			break
		}
		files = append(files, []string{info.Filename, cosa.StringID()})
	}

	data := struct {
		UploadUrl *url.URL
		Files     [][]string
	}{uploadURL, files}
	err = rootTemplate.Execute(w, data)
	if err != nil {
		c.Errorf("%v", err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, _, err := blobstore.ParseUpload(r)
	if err != nil {
		serveError(c, w, err)
		return
	}
	file := blobs["file"]
	if len(file) == 0 {
		c.Errorf("no file uploaded")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func sign(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	g := Greeting{
		Content: r.FormValue("content"),
		Date:    time.Now(),
	}
	if u := user.Current(c); u != nil {
		g.Author = u.String()
	}

	key := datastore.NewIncompleteKey(c, "Greeting", guestbookKey(c))
	_, err := datastore.Put(c, key, &g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
