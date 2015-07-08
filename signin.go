package signin

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"path"

	"appengine"
	"appengine/blobstore"
	"appengine/datastore"
	"appengine/user"
)

//a Tile represents a slot for a team on the buildboard
type Tile struct {
	Name   string
	Desc   string
	Imgref string
}

//the user's logged in status as a struct wrapper
type Status struct {
	LoggedIn bool
}

var status *Status = &Status{LoggedIn: false}

//google app engine init function
func init() {
	//handler for serving static files (css, html)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	//handle google authentication
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
	//handles root view
	http.HandleFunc("/", root)
	//handles creation of new tile
	http.HandleFunc("/submit", submit)

	http.HandleFunc("/serve/", serve)
}

func login(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	//use GAE's redirect to google login if user is not logged in
	if u == nil {
		url, err := user.LoginURL(c, "/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
		return
	}
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
	(*status).LoggedIn = true
	return
}

func logout(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	//requires user to be logged in
	if u != nil {
		url, err := user.LogoutURL(c, "/")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
		return
	} else {
		http.Error(w, "Tried to log out while already logged out", http.StatusInternalServerError)
	}
}

//serves as the ancestor entity for the datastore
//helps with strong consistency for child entities
func tileRootKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "Tile", "home_tiles", 0, nil)
}

//root view
func root(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	//need to check if user is logged in so that the login/logout button
	//is toggled correctly
	if u == nil {
		status.LoggedIn = false
	} else {
		status.LoggedIn = true
	}
	qs := datastore.NewQuery("Tile").Ancestor(tileRootKey(c))
	tiles := make([]Tile, 0, 10)
	if _, err := qs.GetAll(c, &tiles); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Print(tiles)
	//serve the root template
	fp3 := path.Join("templates", "welcome.html")
	templ := template.Must(template.ParseFiles(fp3))

	//obtain a new uploadURL for team photo, for blobstore
	uploadURL, err := blobstore.UploadURL(c, "/submit", nil)
	if err != nil {
		panic("oh no!")
	}
	w.Header().Set("Content-Type", "text/html")
	templ.Execute(w, map[string]interface{}{"Tiles": tiles, "LoggedIn": status, "uploadURL": uploadURL})
}

func submit(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, other, err := blobstore.ParseUpload(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-type", "text/plain")
		io.WriteString(w, "Internal Server Error")
		c.Errorf("%v", err)
		return
	}
	newdata := Tile{
		Name:   string(other["inputName"][0]),
		Desc:   string(other["textArea"][0]),
		Imgref: string(blobs["inputFile"][0].BlobKey),
	}
	key := datastore.NewKey(c, "Tile", newdata.Name, 0, tileRootKey(c))
	_, keyerr := datastore.Put(c, key, &newdata)
	if keyerr != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

//for serving images, using the blobkey we stored in the datastore
func serve(w http.ResponseWriter, r *http.Request) {
	blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}
