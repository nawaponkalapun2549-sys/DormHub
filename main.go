package main

import (
	"bytes"
	"crypto/rand"

	"crypto/subtle"

	"database/sql"

	"encoding/hex"

	"math"

	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"strconv"

	"unicode/utf8"

	"html/template"

	"log"

	"mime/multipart"
	"net/http"

	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/gorilla/sessions"

	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// =========================================================

// SESSION

// =========================================================

// Railway requires SESSION_SECRET; localhost can continue using its legacy development key.
var store = newSessionStore()

func newSessionStore() *sessions.CookieStore {
    secret := os.Getenv("SESSION_SECRET")
    if os.Getenv("RAILWAY_ENVIRONMENT") != "" && len(secret) < 32 {
        log.Fatal("SESSION_SECRET must contain at least 32 characters on Railway")
    }
    if secret == "" { secret = "dormhub-secret-key-change-this-later" } // localhost only
    s := sessions.NewCookieStore([]byte(secret))
    s.Options = &sessions.Options{Path:"/", MaxAge:86400, HttpOnly:true, SameSite:http.SameSiteLaxMode, Secure: isHTTPSDeployment()}
    return s
}

func isHTTPSDeployment() bool {
    return os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("COOKIE_SECURE") == "true"
}

// UPLOAD_DIR should point at a mounted persistent volume, e.g. /data/uploads.
func uploadedImageDir() string {
    root := os.Getenv("UPLOAD_DIR")
    if root != "" { return filepath.Join(root, "dorms") }
    return filepath.Join("static", "uploads", "dorms")
}
func uploadedDocumentDir() string {
    root := os.Getenv("UPLOAD_DIR")
    if root != "" { return filepath.Join(root, "landlord_docs") }
    return filepath.Join("private_uploads", "landlord_docs")
}

func databaseDSN() string {
    if dsn := os.Getenv("DB_DSN"); dsn != "" { return dsn }
    if os.Getenv("RAILWAY_ENVIRONMENT") == "" && os.Getenv("MYSQLHOST") == "" {
        return "root:@tcp(127.0.0.1:3306)/DormHub?charset=utf8mb4&parseTime=True&loc=Local"
    }
    host:=os.Getenv("MYSQLHOST")
    port:=os.Getenv("MYSQLPORT")
    user:=os.Getenv("MYSQLUSER")
    pass:=os.Getenv("MYSQLPASSWORD")
    database:=os.Getenv("MYSQLDATABASE")
    if host=="" || port=="" || user=="" || database=="" { log.Fatal("Missing MySQL variables: MYSQLHOST, MYSQLPORT, MYSQLUSER, MYSQLDATABASE") }
    cfg:=mysql.NewConfig()
    cfg.User=user; cfg.Passwd=pass; cfg.Net="tcp"; cfg.Addr=host+":"+port; cfg.DBName=database
    cfg.ParseTime=true
    cfg.Params=map[string]string{"charset":"utf8mb4"}
    return cfg.FormatDSN()
}

// =========================================================

// MAIN

// =========================================================

func main() {

	// =========================

	// DATABASE

	// =========================

	dsn := databaseDSN()

	var err error

	db, err = sql.Open("mysql", dsn)

	if err != nil {

		log.Fatal("เน€เธเธดเธ” Database เนเธกเนเธชเธณเน€เธฃเนเธ:", err)

	}

	if err = db.Ping(); err != nil {

		log.Fatal("เน€เธเธทเนเธญเธกเธ•เนเธญ Database เนเธกเนเธชเธณเน€เธฃเนเธ:", err)

	}

	log.Println("=================================")

	log.Println("โ… DormHub Database Connected")

	log.Println("=================================")

	// =========================

	// ROUTES

	// =========================

	http.HandleFunc("/", homeHandler)

	http.HandleFunc("/register", registerHandler)

	http.HandleFunc("/login", loginHandler)

	http.HandleFunc("/logout", logoutHandler)

	// Dashboard

	http.HandleFunc("/dashboard", dashboardHandler)
	http.HandleFunc("/my-requests", myRequestsHandler)
	http.HandleFunc("/favorites", favoritesHandler)
	http.HandleFunc("/favorites/toggle", favoriteToggleHandler)
	http.HandleFunc("/reviews/save", reviewSaveHandler)
	http.HandleFunc("/reviews/delete", reviewDeleteHandler)
	http.HandleFunc("/my-reviews", myReviewsHandler)
	// STEP 13 - ADMIN (database role verified on every request)
	http.HandleFunc("/admin", adminDashboardHandler)
	http.HandleFunc("/admin/users", adminUsersHandler)
	http.HandleFunc("/admin/dorms", adminDormsHandler)
	http.HandleFunc("/admin/requests", adminRequestsHandler)
	http.HandleFunc("/admin/landlord-applications", adminLandlordApplicationsHandler)
	http.HandleFunc("/admin/landlord-document", adminLandlordDocumentHandler)
	http.HandleFunc("/landlord/application-status", landlordApplicationStatusHandler)

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/dorm", dormDetailHandler)
	http.HandleFunc("/rental/request", rentalRequestHandler)

	http.HandleFunc("/landlord", landlordHandler)
	http.HandleFunc("/landlord/requests", landlordRequestsHandler)

	http.HandleFunc("/landlord/add", landlordAddHandler)

	http.HandleFunc("/landlord/rooms/add", landlordRoomAddHandler)

	http.HandleFunc("/landlord/rooms", landlordRoomsHandler)
	http.HandleFunc("/landlord/rooms/edit", landlordRoomEditHandler)
	http.HandleFunc("/landlord/rooms/delete", landlordRoomDeleteHandler)

    // Persistent upload images take precedence over the bundled static server.
    http.Handle("/static/uploads/dorms/", http.StripPrefix("/static/uploads/dorms/", http.FileServer(http.Dir(uploadedImageDir()))))

	// =========================

	// STATIC FILES

	// =========================

	http.Handle(

		"/static/",

		http.StripPrefix(

			"/static/",

			http.FileServer(http.Dir("static")),
		),
	)

	// =========================

	// SERVER

	// =========================

	log.Println("๐€ DormHub server starting")

	log.Println("=================================")

	port := os.Getenv("PORT")
    if port == "" { port = "8080" }
    log.Printf("Listening on port %s", port)
    err = http.ListenAndServe(":"+port, nil)

	if err != nil {

		log.Fatal("Server Error:", err)

	}

}

// =========================================================

// HOME

// =========================================================

func homeHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {

		http.NotFound(w, r)

		return

	}

	if r.Method != http.MethodGet {

		http.Error(

			w,

			"Method Not Allowed",

			http.StatusMethodNotAllowed,
		)

		return

	}

	tmpl, err := template.ParseFiles(

		"templates/home.html",
	)

	if err != nil {

		log.Println("Home Template Error:", err)

		http.Error(

			w,

			"เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเน€เธงเนเธเนเธ”เน",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// GET SESSION

	// =========================

	session, err := store.Get(

		r,

		"dormhub-session",
	)

	if err != nil {

		log.Println("Session Error:", err)

		tmpl.Execute(w, nil)

		return

	}

	// =========================

	// SESSION DATA

	// =========================

	var data struct {
		IsLoggedIn bool

		UserID int

		Name string

		Role string
	}

	userID, ok := session.Values["user_id"]

	if ok && userID != nil {

		switch v := userID.(type) {

		case int:

			data.UserID = v

			data.IsLoggedIn = true

		case int64:

			data.UserID = int(v)

			data.IsLoggedIn = true

		}

	}

	if name, ok := session.Values["name"].(string); ok {

		data.Name = name

	}

	if role, ok := session.Values["role"].(string); ok {

		data.Role = role

	}

	// =========================

	// RENDER

	// =========================

	err = tmpl.Execute(w, data)

	if err != nil {

		log.Println("Home Template Execute Error:", err)

	}

}

// =========================================================

// REGISTER

// =========================================================

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/register" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		renderRegister(w, "")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// A small upload limit for demo documents, including form overhead.
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		showRegisterError(w, "เธเนเธญเธกเธนเธฅเธซเธฃเธทเธญเน€เธญเธเธชเธฒเธฃเธกเธตเธเธเธฒเธ”เนเธซเธเนเน€เธเธดเธเธเธณเธซเธเธ” (เธชเธนเธเธชเธธเธ” 3 MB)")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	name := strings.TrimSpace(r.FormValue("name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")
	accountType := r.FormValue("account_type")
	if accountType != "user" && accountType != "landlord" {
		showRegisterError(w, "เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธเธฃเธฐเน€เธ เธ—เธเธฑเธเธเธต user เธซเธฃเธทเธญ landlord")
		return
	}
	if name == "" || len([]rune(name)) > 100 || len(phone) != 10 || password == "" || confirm == "" {
		showRegisterError(w, "เธเธฃเธธเธ“เธฒเธ•เธฃเธงเธเธชเธญเธเธเธทเนเธญ เน€เธเธญเธฃเนเนเธ—เธฃ 10 เธซเธฅเธฑเธ เนเธฅเธฐเธฃเธซเธฑเธชเธเนเธฒเธเนเธซเนเธเธฃเธ")
		return
	}
	for _, c := range phone {
		if c < '0' || c > '9' {
			showRegisterError(w, "เน€เธเธญเธฃเนเนเธ—เธฃเธ•เนเธญเธเน€เธเนเธเธ•เธฑเธงเน€เธฅเธ 10 เธซเธฅเธฑเธ")
			return
		}
	}
	if len(password) < 6 || password != confirm {
		showRegisterError(w, "เธฃเธซเธฑเธชเธเนเธฒเธเธ•เนเธญเธเธกเธตเธญเธขเนเธฒเธเธเนเธญเธข 6 เธ•เธฑเธงเธญเธฑเธเธฉเธฃเนเธฅเธฐเธขเธทเธเธขเธฑเธเนเธซเนเธ•เธฃเธเธเธฑเธ")
		return
	}

	var document multipart.File
	var header *multipart.FileHeader
	dormName := ""
	if accountType == "landlord" {
		dormName = strings.TrimSpace(r.FormValue("dorm_name"))
		if dormName == "" || len([]rune(dormName)) > 150 {
			showRegisterError(w, "เธเธฃเธธเธ“เธฒเธเธฃเธญเธเธเธทเนเธญเธซเธญเธเธฑเธเธ•เธฑเธงเธญเธขเนเธฒเธเนเธกเนเน€เธเธดเธ 150 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ")
			return
		}
		var err error
		document, header, err = r.FormFile("proof_document")
		if err != nil {
			showRegisterError(w, "เธเธฃเธธเธ“เธฒเนเธเธเนเธเธฅเนเน€เธญเธเธชเธฒเธฃเธ—เธ”เธชเธญเธ")
			return
		}
		defer document.Close()
		if header.Size == 0 || header.Size > 3<<20 {
			showRegisterError(w, "เน€เธญเธเธชเธฒเธฃเธ•เนเธญเธเธกเธตเธเธเธฒเธ” 1 เนเธเธ•เน เธ–เธถเธ 3 MB")
			return
		}
	}
	var existing int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE phone=?", phone).Scan(&existing); err != nil {
		log.Println("Register user check error:", err)
		showRegisterError(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเน€เธเธญเธฃเนเนเธ—เธฃเนเธ”เน")
		return
	}
	if existing != 0 {
		showRegisterError(w, "เน€เธเธญเธฃเนเนเธ—เธฃเธเธตเนเธ–เธนเธเนเธเนเธเธฒเธเนเธฅเนเธง")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		showRegisterError(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเธฃเธซเธฑเธชเธเนเธฒเธเนเธ”เน")
		return
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Register tx error:", err)
		showRegisterError(w, "เธเธฒเธเธเนเธญเธกเธนเธฅเนเธกเนเธเธฃเนเธญเธกเนเธเนเธเธฒเธ")
		return
	}
	defer tx.Rollback()
	// Everybody starts as user. Only a reviewing admin can grant landlord role.
	inserted, err := tx.ExecContext(r.Context(), "INSERT INTO users (name,phone,password_hash,role) VALUES (?,?,?,'user')", name, phone, string(hashed))
	if err != nil {
		log.Println("Register insert error:", err)
		showRegisterError(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเธเธฑเธเธเธตเนเธ”เน (เน€เธเธญเธฃเนเธญเธฒเธเธเนเธณ)")
		return
	}
	userID, err := inserted.LastInsertId()
	if err != nil {
		showRegisterError(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเธเธฑเธเธเธตเนเธ”เน")
		return
	}

	savedPath := ""
	if accountType == "landlord" {
		savedPath, err = saveDemoLandlordDocument(document, header)
		if err != nil {
			showRegisterError(w, err.Error())
			return
		}
		defer func() {
			if savedPath != "" {
				_ = os.Remove(savedPath)
			}
		}()
		_, err = tx.ExecContext(r.Context(), `INSERT INTO landlord_applications
            (user_id,dorm_name,document_path,original_filename,status)
            VALUES (?,?,?,?,'pending')`, userID, dormName, filepath.ToSlash(savedPath), filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/")))
		if err != nil {
			log.Println("Landlord application insert error:", err)
			showRegisterError(w, "เธเธฑเธเธ—เธถเธเธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ เธเธฃเธธเธ“เธฒเธ•เธฃเธงเธเธชเธญเธเธงเนเธฒ Import SQL เธเธฑเนเธเธ•เธญเธ 15 เนเธฅเนเธง")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Println("Register commit error:", err)
		showRegisterError(w, "เธเธฑเธเธ—เธถเธเธเธฒเธฃเธชเธกเธฑเธเธฃเนเธกเนเธชเธณเน€เธฃเนเธ")
		return
	}
	savedPath = "" // Keep the document only after successful DB commit.
	destination := "/login?registered=1"
	if accountType == "landlord" {
		destination += "&landlord=pending"
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

func renderRegister(w http.ResponseWriter, message string) {
	t, err := template.ParseFiles("templates/register.html")
	if err != nil {
		http.Error(w, "เนเธซเธฅเธ”เธซเธเนเธฒเธชเธกเธฑเธเธฃเนเธกเนเนเธ”เน", 500)
		return
	}
	if err := t.Execute(w, struct{ Error string }{Error: message}); err != nil {
		log.Println("Register render error:", err)
	}
}
func showRegisterError(w http.ResponseWriter, message string) { renderRegister(w, message) }

// =========================================================

// LOGIN

// =========================================================

func loginHandler(w http.ResponseWriter, r *http.Request) {

	// =========================

	// GET

	// =========================

	if r.Method == http.MethodGet {

		tmpl, err := template.ParseFiles(

			"templates/login.html",
		)

		if err != nil {

			http.Error(

				w,

				"เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒ Login เนเธ”เน",

				http.StatusInternalServerError,
			)

			return

		}

		data := struct {
			Error string

			Success string
		}{

			Error: "",

			Success: "",
		}

		if r.URL.Query().Get("registered") == "1" {
			data.Success = "เธชเธกเธฑเธเธฃเธชเธกเธฒเธเธดเธเธชเธณเน€เธฃเนเธ! เธเธฃเธธเธ“เธฒเน€เธเนเธฒเธชเธนเนเธฃเธฐเธเธ"
			if r.URL.Query().Get("landlord") == "pending" {
				data.Success = "เธชเธกเธฑเธเธฃเธเธฑเธเธเธตเน€เธเนเธฒเธเธญเธเธซเธญเธชเธณเน€เธฃเนเธ! เน€เธญเธเธชเธฒเธฃเธ•เธฑเธงเธญเธขเนเธฒเธเธญเธขเธนเนเธฃเธฐเธซเธงเนเธฒเธเธฃเธญ Admin เธ•เธฃเธงเธเธชเธญเธ เธเธฃเธธเธ“เธฒ Login เน€เธเธทเนเธญเธ”เธนเธชเธ–เธฒเธเธฐ"
			}
		}

		err = tmpl.Execute(w, data)

		if err != nil {

			log.Println(

				"Login Template Error:",

				err,
			)

		}

		return

	}

	// =========================

	// POST

	// =========================

	if r.Method != http.MethodPost {

		http.Error(

			w,

			"Method Not Allowed",

			http.StatusMethodNotAllowed,
		)

		return

	}

	err := r.ParseForm()

	if err != nil {

		http.Error(

			w,

			"เธเนเธญเธกเธนเธฅเนเธกเนเธ–เธนเธเธ•เนเธญเธ",

			http.StatusBadRequest,
		)

		return

	}

	// =========================

	// FORM DATA

	// =========================

	phone := strings.TrimSpace(

		r.FormValue("phone"),
	)

	password := r.FormValue("password")

	// =========================

	// CHECK EMPTY

	// =========================

	if phone == "" || password == "" {

		showLoginError(

			w,

			"เธเธฃเธธเธ“เธฒเธเธฃเธญเธเน€เธเธญเธฃเนเนเธ—เธฃเนเธฅเธฐเธฃเธซเธฑเธชเธเนเธฒเธ",
		)

		return

	}

	// =========================

	// FIND USER BY PHONE

	// =========================

	var (
		id int

		name string

		passwordHash string

		role string
	)

	err = db.QueryRow(`















        SELECT















            id,















            name,















            password_hash,















            role















        FROM users















        WHERE phone = ?















    `,

		phone,
	).Scan(

		&id,

		&name,

		&passwordHash,

		&role,
	)

	if err != nil {

		log.Println(

			"Login User Error:",

			err,
		)

		showLoginError(

			w,

			"เน€เธเธญเธฃเนเนเธ—เธฃเธซเธฃเธทเธญเธฃเธซเธฑเธชเธเนเธฒเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ",
		)

		return

	}

	// =========================

	// CHECK PASSWORD

	// =========================

	err = bcrypt.CompareHashAndPassword(

		[]byte(passwordHash),

		[]byte(password),
	)

	if err != nil {

		showLoginError(

			w,

			"เน€เธเธญเธฃเนเนเธ—เธฃเธซเธฃเธทเธญเธฃเธซเธฑเธชเธเนเธฒเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ",
		)

		return

	}

	// =========================

	// GET SESSION

	// =========================

	session, err := store.Get(

		r,

		"dormhub-session",
	)

	if err != nil {

		log.Println(

			"Session Get Error:",

			err,
		)

		http.Error(

			w,

			"เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธ Session เนเธ”เน",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// SAVE SESSION

	// =========================

	session.Values["user_id"] = id

	session.Values["name"] = name

	session.Values["role"] = role

	session.Options = &sessions.Options{

		Path: "/",

		MaxAge: 86400,

		HttpOnly: true,

		SameSite: http.SameSiteLaxMode,

		Secure: isHTTPSDeployment(),
	}

	err = session.Save(

		r,

		w,
	)

	if err != nil {

		log.Println(

			"Session Save Error:",

			err,
		)

		http.Error(

			w,

			"เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// LOG

	// =========================

	log.Printf(

		"Login เธชเธณเน€เธฃเนเธ: ID=%d Name=%s Phone=%s Role=%s",

		id,

		name,

		phone,

		role,
	)

	// Pending/rejected landlord applications are shown a clear status page.
	if role == "user" {
		var appStatus string
		e := db.QueryRowContext(r.Context(), "SELECT status FROM landlord_applications WHERE user_id=?", id).Scan(&appStatus)
		if e == nil {
			http.Redirect(w, r, "/landlord/application-status", http.StatusSeeOther)
			return
		}
		if e != sql.ErrNoRows {
			log.Println("Application status lookup error:", e)
		}
	}

	// =========================

	// HOME

	// =========================

	http.Redirect(

		w,

		r,

		"/",

		http.StatusSeeOther,
	)

}

// =========================================================

// LOGIN ERROR

// =========================================================

func showLoginError(

	w http.ResponseWriter,

	message string,

) {

	tmpl, err := template.ParseFiles(

		"templates/login.html",
	)

	if err != nil {

		http.Error(

			w,

			message,

			http.StatusInternalServerError,
		)

		return

	}

	// login.html accesses both .Error and .Success.
	// Always supply both fields, including on invalid-login responses.
	data := struct {
		Error   string
		Success string
	}{
		Error:   message,
		Success: "",
	}

	err = tmpl.Execute(

		w,

		data,
	)

	if err != nil {

		log.Println(

			"Login Template Error:",

			err,
		)

	}

}

// =========================================================

// LOGOUT

// =========================================================

func logoutHandler(

	w http.ResponseWriter,

	r *http.Request,

) {

	if r.Method != http.MethodGet {

		http.Error(

			w,

			"Method Not Allowed",

			http.StatusMethodNotAllowed,
		)

		return

	}

	session, err := store.Get(

		r,

		"dormhub-session",
	)

	if err != nil {

		http.Error(

			w,

			"Session error",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// CLEAR SESSION

	// =========================

	session.Values["user_id"] = nil

	session.Values["name"] = nil

	session.Values["role"] = nil

	session.Options.MaxAge = -1

	err = session.Save(

		r,

		w,
	)

	if err != nil {

		http.Error(

			w,

			"Logout error",

			http.StatusInternalServerError,
		)

		return

	}

	log.Println("Logout เธชเธณเน€เธฃเนเธ")

	http.Redirect(

		w,

		r,

		"/",

		http.StatusSeeOther,
	)

}

// =========================================================

// DASHBOARD

// =========================================================

func dashboardHandler(

	w http.ResponseWriter,

	r *http.Request,

) {

	if r.Method != http.MethodGet {

		http.Error(

			w,

			"Method Not Allowed",

			http.StatusMethodNotAllowed,
		)

		return

	}

	// =========================

	// GET SESSION

	// =========================

	session, err := store.Get(

		r,

		"dormhub-session",
	)

	if err != nil {

		log.Println(

			"Dashboard Session Error:",

			err,
		)

		http.Redirect(

			w,

			r,

			"/login",

			http.StatusSeeOther,
		)

		return

	}

	// =========================

	// CHECK LOGIN

	// =========================

	userIDValue, ok :=

		session.Values["user_id"]

	if !ok || userIDValue == nil {

		http.Redirect(

			w,

			r,

			"/login",

			http.StatusSeeOther,
		)

		return

	}

	// =========================

	// CONVERT USER ID

	// =========================

	var userID int

	switch v := userIDValue.(type) {

	case int:

		userID = v

	case int64:

		userID = int(v)

	default:

		log.Println(

			"Invalid User ID in Session",
		)

		http.Redirect(

			w,

			r,

			"/login",

			http.StatusSeeOther,
		)

		return

	}

	// =========================

	// GET NAME

	// =========================

	name, ok :=

		session.Values["name"].(string)

	if !ok || name == "" {

		name = "เธเธนเนเนเธเนเธเธฒเธ"

	}

	// =========================

	// GET ROLE

	// =========================

	role, ok :=

		session.Values["role"].(string)

	if !ok || role == "" {

		role = "user"

	}

	// =========================

	// LOAD TEMPLATE

	// =========================

	tmpl, err := template.ParseFiles(

		"templates/dashboard.html",
	)

	if err != nil {

		log.Println(

			"Dashboard Template Error:",

			err,
		)

		http.Error(

			w,

			"เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ” Dashboard เนเธ”เน",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// DATA

	// =========================

	data := struct {
		UserID int

		Name string

		Role string
	}{

		UserID: userID,

		Name: name,

		Role: role,
	}

	// =========================

	// RENDER

	// =========================

	err = tmpl.Execute(

		w,

		data,
	)

	if err != nil {

		log.Println(

			"Dashboard Template Execute Error:",

			err,
		)

	}

}

// DormSearchResult is one available dorm returned from the database.

type DormSearchResult struct {
	ID int

	Name string

	Address string

	Description string

	Image string

	MinPrice float64

	AvailableRooms int
}

type SearchPageData struct {
	IsLoggedIn bool

	Name string

	Role string

	Query string

	Location string

	Price string

	RoomType string

	Dorms []DormSearchResult

	Count int
}

// searchHandler searches available rooms and shows the matching dormitories.

func searchHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	data := SearchPageData{

		Query: strings.TrimSpace(r.URL.Query().Get("q")),

		Location: r.URL.Query().Get("location"),

		Price: r.URL.Query().Get("price"),

		RoomType: r.URL.Query().Get("room_type"),

		Dorms: []DormSearchResult{},
	}

	// Display the same signed-in navigation as the homepage.

	session, err := store.Get(r, "dormhub-session")

	if err == nil {

		if id, ok := session.Values["user_id"].(int); ok && id > 0 {

			data.IsLoggedIn = true

		} else if id, ok := session.Values["user_id"].(int64); ok && id > 0 {

			data.IsLoggedIn = true

		}

		if data.IsLoggedIn {

			data.Name, _ = session.Values["name"].(string)

			data.Role, _ = session.Values["role"].(string)

		}

	}

	// Only predefined filter clauses are included; user values use placeholders.

	query := `







        SELECT d.id, d.name, d.address, COALESCE(d.description, ''),







               COALESCE(d.image, ''), MIN(r.price), COUNT(r.id)







        FROM dormitories d







        JOIN rooms r ON r.dormitory_id = d.id AND r.status = 'available'







        WHERE 1 = 1`

	args := make([]any, 0, 8)

	if data.Query != "" {

		query += " AND (d.name LIKE ? OR d.address LIKE ?)"

		keyword := "%" + data.Query + "%"

		args = append(args, keyword, keyword)

	}

	switch data.Location {

	case "เธกเธซเธฒเธชเธฒเธฃเธเธฒเธก":

		query += " AND d.address LIKE ?"

		args = append(args, "%เธกเธซเธฒเธชเธฒเธฃเธเธฒเธก%")

	case "เธกเธกเธช":

		query += " AND (d.address LIKE ? OR d.address LIKE ?)"

		args = append(args, "%เธกเธกเธช%", "%เธกเธซเธฒเธงเธดเธ—เธขเธฒเธฅเธฑเธขเธกเธซเธฒเธชเธฒเธฃเธเธฒเธก%")

	default:

		data.Location = ""

	}

	switch data.Price {

	case "under3000":

		query += " AND r.price < ?"

		args = append(args, 3000)

	case "3000-5000":

		query += " AND r.price BETWEEN ? AND ?"

		args = append(args, 3000, 5000)

	case "over5000":

		query += " AND r.price > ?"

		args = append(args, 5000)

	default:

		data.Price = ""

	}

	switch data.RoomType {

	case "air":

		query += " AND (LOWER(r.room_type) LIKE ? OR r.room_type LIKE ?)"

		args = append(args, "%air%", "%เนเธญเธฃเน%")

	case "fan":

		query += " AND (LOWER(r.room_type) LIKE ? OR r.room_type LIKE ?)"

		args = append(args, "%fan%", "%เธเธฑเธ”เธฅเธก%")

	default:

		data.RoomType = ""

	}

	query += `







        GROUP BY d.id, d.name, d.address, d.description, d.image







        ORDER BY d.id DESC







        LIMIT 100`

	rows, err := db.QueryContext(r.Context(), query, args...)

	if err != nil {

		log.Println("Search Query Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธเนเธเธซเธฒเธซเธญเธเธฑเธ", http.StatusInternalServerError)

		return

	}

	defer rows.Close()

	for rows.Next() {

		var dorm DormSearchResult

		if err := rows.Scan(&dorm.ID, &dorm.Name, &dorm.Address,

			&dorm.Description, &dorm.Image, &dorm.MinPrice,

			&dorm.AvailableRooms); err != nil {

			log.Println("Search Scan Error:", err)

			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

			return

		}

		data.Dorms = append(data.Dorms, dorm)

	}

	if err := rows.Err(); err != nil {

		log.Println("Search Rows Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธญเนเธฒเธเธเธฅเธเนเธเธซเธฒ", http.StatusInternalServerError)

		return

	}

	data.Count = len(data.Dorms)

	tmpl, err := template.ParseFiles("templates/search.html")

	if err != nil {

		log.Println("Search Template Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธเนเธเธซเธฒเนเธ”เน", http.StatusInternalServerError)

		return

	}

	if err := tmpl.Execute(w, data); err != nil {

		log.Println("Search Template Render Error:", err)

	}

}

// =========================================================

// LANDLORD DASHBOARD

// =========================================================

// =========================================================

// LANDLORD DASHBOARD

// =========================================================

func landlordHandler(w http.ResponseWriter, r *http.Request) {

	// เธ•เธฃเธงเธเธชเธญเธ URL

	if r.URL.Path != "/landlord" {

		http.NotFound(w, r)

		return

	}

	// เธญเธเธธเธเธฒเธ•เน€เธเธเธฒเธฐ GET

	if r.Method != http.MethodGet {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	// =========================

	// เธ•เธฃเธงเธเธชเธญเธ SESSION

	// =========================

	session, err := store.Get(r, "dormhub-session")

	if err != nil {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	var userID int

	switch v := session.Values["user_id"].(type) {

	case int:

		userID = v

	case int64:

		userID = int(v)

	default:

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if userID <= 0 {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	// =========================

	// เธ•เธฃเธงเธเธชเธดเธ—เธเธดเนเธเธฒเธ DATABASE

	// =========================

	var name string

	var role string

	err = db.QueryRowContext(

		r.Context(),

		"SELECT name, role FROM users WHERE id = ?",

		userID,
	).Scan(&name, &role)

	if err == sql.ErrNoRows {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if err != nil {

		log.Println("Landlord User Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

		return

	}

	// เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ

	if role != "landlord" {

		http.Error(

			w,

			"403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ",

			http.StatusForbidden,
		)

		return

	}

	// =========================

	// เนเธเธฃเธเธชเธฃเนเธฒเธเธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธ

	// =========================

	type LandlordDorm struct {
		ID int

		Name string

		Address string

		RoomCount int
	}

	type LandlordPageData struct {
		Name string

		DormCount int

		RoomCount int

		PendingCount int

		Dorms []LandlordDorm
	}

	data := LandlordPageData{

		Name: name,

		Dorms: []LandlordDorm{},
	}

	// =========================

	// เธ”เธถเธเน€เธเธเธฒเธฐเธซเธญเธเธฑเธเธเธญเธเน€เธเนเธฒเธเธญเธ

	// =========================

	rows, err := db.QueryContext(

		r.Context(),

		`



        SELECT



            d.id,



            d.name,



            d.address,



            COUNT(r.id) AS room_count



        FROM dormitories d



        LEFT JOIN rooms r



            ON r.dormitory_id = d.id



        WHERE d.owner_id = ?



        GROUP BY d.id, d.name, d.address



        ORDER BY d.id DESC



        `,

		userID,
	)

	if err != nil {

		log.Println("Landlord Dorm Query Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	for rows.Next() {

		var dorm LandlordDorm

		err := rows.Scan(

			&dorm.ID,

			&dorm.Name,

			&dorm.Address,

			&dorm.RoomCount,
		)

		if err != nil {

			rows.Close()

			log.Println("Landlord Dorm Scan Error:", err)

			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

			return

		}

		data.Dorms = append(data.Dorms, dorm)

		data.RoomCount += dorm.RoomCount

	}

	if err := rows.Err(); err != nil {

		rows.Close()

		log.Println("Landlord Dorm Rows Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธ", http.StatusInternalServerError)

		return

	}

	rows.Close()

	// เธเธณเธเธงเธเธซเธญเธเธฑเธเธเธญเธเธ•เธฑเธงเน€เธญเธ

	data.DormCount = len(data.Dorms)

	// =========================

	// เธเธฑเธเธเธณเธเธญเน€เธเนเธฒเธ—เธตเนเธฃเธญเธ•เธฃเธงเธเธชเธญเธ

	// =========================

	err = db.QueryRowContext(

		r.Context(),

		`



        SELECT COUNT(*)



        FROM rental_requests rr



        INNER JOIN rooms r



            ON rr.room_id = r.id



        INNER JOIN dormitories d



            ON r.dormitory_id = d.id



        WHERE d.owner_id = ?



          AND rr.status = 'pending'



        `,

		userID,
	).Scan(&data.PendingCount)

	if err != nil {

		log.Println("Landlord Pending Count Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)

		return

	}

	// =========================

	// เนเธซเธฅเธ” LANDLORD.HTML

	// =========================

	tmpl, err := template.ParseFiles(

		"templates/landlord.html",
	)

	if err != nil {

		log.Println("Landlord Template Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒ Landlord เนเธ”เน", http.StatusInternalServerError)

		return

	}

	// =========================

	// เนเธชเธ”เธเธซเธเนเธฒ Dashboard

	// =========================

	err = tmpl.Execute(w, data)

	if err != nil {

		log.Println("Landlord Render Error:", err)

	}

}

// =========================================================

// LANDLORD - ADD DORMITORY

// =========================================================

func landlordAddHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/landlord/add" {

		http.NotFound(w, r)

		return

	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	// Identify the logged-in user using the signed session cookie.

	session, err := store.Get(r, "dormhub-session")

	if err != nil {

		log.Println("Landlord Add Session Error:", err)

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	var userID int

	switch v := session.Values["user_id"].(type) {

	case int:

		userID = v

	case int64:

		userID = int(v)

	default:

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if userID <= 0 {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	// Always check the real role in the database, not the browser or form.

	var role string

	err = db.QueryRowContext(r.Context(),

		"SELECT role FROM users WHERE id = ?", userID,
	).Scan(&role)

	if err == sql.ErrNoRows {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if err != nil {

		log.Println("Landlord Add Role Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(w, "403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)

		return

	}

	// GET: generate a random CSRF token bound to this user's session.

	if r.Method == http.MethodGet {

		tokenBytes := make([]byte, 32)

		if _, err := rand.Read(tokenBytes); err != nil {

			log.Println("Landlord Add CSRF Generation Error:", err)

			http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

			return

		}

		token := hex.EncodeToString(tokenBytes)

		session.Values["landlord_add_csrf"] = token

		if err := session.Save(r, w); err != nil {

			log.Println("Landlord Add Session Save Error:", err)

			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)

			return

		}

		renderLandlordAdd(w, "", token)

		return

	}

	// POST: keep form payloads small.

	// Multipart is necessary to upload an image from the user's computer.
	// Limit total request size to 8 MiB (including other form fields).
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)

	if err := r.ParseMultipartForm(2 << 20); err != nil {

		http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธเธซเธฃเธทเธญเธกเธตเธเธเธฒเธ”เนเธซเธเนเน€เธเธดเธเนเธ", http.StatusBadRequest)

		return

	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	savedToken, ok := session.Values["landlord_add_csrf"].(string)

	submittedToken := r.PostFormValue("csrf_token")

	if !ok || savedToken == "" || subtle.ConstantTimeCompare(

		[]byte(savedToken), []byte(submittedToken),
	) != 1 {

		http.Error(w, "403 Forbidden: CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธเธญเธฃเนเธกเนเธซเธกเน", http.StatusForbidden)

		return

	}

	dormName := strings.TrimSpace(r.PostFormValue("dorm_name"))

	address := strings.TrimSpace(r.PostFormValue("address"))

	description := strings.TrimSpace(r.PostFormValue("description"))

	if dormName == "" || address == "" {

		renderLandlordAdd(w, "เธเธฃเธธเธ“เธฒเธเธฃเธญเธเธเธทเนเธญเธซเธญเธเธฑเธเนเธฅเธฐเธ—เธตเนเธญเธขเธนเน", savedToken)

		return

	}

	if utf8.RuneCountInString(dormName) > 150 {

		renderLandlordAdd(w, "เธเธทเนเธญเธซเธญเธเธฑเธเธ•เนเธญเธเนเธกเนเน€เธเธดเธ 150 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ", savedToken)

		return

	}

	if utf8.RuneCountInString(address) > 2000 {

		renderLandlordAdd(w, "เธ—เธตเนเธญเธขเธนเนเธขเธฒเธงเน€เธเธดเธเธเธณเธซเธเธ”", savedToken)

		return

	}

	if utf8.RuneCountInString(description) > 5000 {

		renderLandlordAdd(w, "เธฃเธฒเธขเธฅเธฐเน€เธญเธตเธขเธ”เธซเธญเธเธฑเธเธขเธฒเธงเน€เธเธดเธเธเธณเธซเธเธ”", savedToken)

		return

	}

	// Optional local photo: only upload after CSRF and the text fields pass validation.
	imageURL := ""
	imageFile, imageHeader, fileErr := r.FormFile("image_file")
	if fileErr != nil && !errors.Is(fileErr, http.ErrMissingFile) {
		renderLandlordAdd(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเนเธเธฅเนเธฃเธนเธเธ เธฒเธเนเธ”เน เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเนเธเธฅเนเธญเธตเธเธเธฃเธฑเนเธ", savedToken)
		return
	}
	if fileErr == nil {
		defer imageFile.Close()
		// Protect against oversized uploads before touching the file system.
		if imageHeader.Size > 5<<20 {
			renderLandlordAdd(w, "เธฃเธนเธเธ เธฒเธเธ•เนเธญเธเธกเธตเธเธเธฒเธ”เนเธกเนเน€เธเธดเธ 5 MB", savedToken)
			return
		}
		imageURL, err = saveDormImage(imageFile)
		if err != nil {
			log.Println("Landlord Add Image Upload Error:", err)
			renderLandlordAdd(w, "เธญเธฑเธเนเธซเธฅเธ”เธฃเธนเธเนเธกเนเธชเธณเน€เธฃเนเธ เนเธเนเนเธเธฅเน JPG, PNG, GIF เธซเธฃเธทเธญ WebP เธเธเธฒเธ”เนเธกเนเน€เธเธดเธ 5 MB", savedToken)
			return
		}
	}

	// Ownership is taken ONLY from the authenticated session.

	_, err = db.ExecContext(r.Context(), `

        INSERT INTO dormitories (owner_id, name, address, description, image)

        VALUES (?, ?, ?, ?, ?)

    `, userID, dormName, address, description, imageURL)

	if err != nil {

		// Prevent abandoned images if the database insert fails.
		if imageURL != "" {
			_ = os.Remove(filepath.Join(uploadedImageDir(), filepath.Base(imageURL)))
		}
		log.Println("Landlord Add Insert Error:", err)

		renderLandlordAdd(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธเธซเธญเธเธฑเธเนเธ”เน เธเธฃเธธเธ“เธฒเธฅเธญเธเธญเธตเธเธเธฃเธฑเนเธ", savedToken)

		return

	}

	// Single-use token helps prevent accidental duplicate submissions.

	delete(session.Values, "landlord_add_csrf")

	if err := session.Save(r, w); err != nil {

		log.Println("Landlord Add CSRF Clear Error:", err)

		// Data was already inserted; redirect rather than risk another POST.

	}

	log.Printf("Landlord %d added dormitory ID via form: %s", userID, dormName)

	http.Redirect(w, r, "/landlord?added=1", http.StatusSeeOther)

}

// saveDormImage stores a validated image under the existing /static/ file server.
// Only a generated server-side file name is used: the client file name is ignored.
// The returned /static/... URL fits the existing dormitories.image VARCHAR(255).
func saveDormImage(upload multipart.File) (string, error) {
	const maxImageBytes int64 = 5 << 20
	folder := uploadedImageDir()
	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(folder, ".dorm-upload-*")
	if err != nil {
		return "", err
	}
	tempName := file.Name()
	defer os.Remove(tempName)

	// MaxBytesReader protects the request; LimitReader also enforces the image limit.
	written, copyErr := io.Copy(file, io.LimitReader(upload, maxImageBytes+1))
	if copyErr != nil {
		file.Close()
		return "", copyErr
	}
	if written == 0 || written > maxImageBytes {
		file.Close()
		return "", errors.New("invalid image size")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return "", err
	}

	// Detect file format from file contents, not the extension sent by the user.
	var head [512]byte
	n, err := file.Read(head[:])
	if err != nil && err != io.EOF {
		file.Close()
		return "", err
	}
	contentType := http.DetectContentType(head[:n])
	extension := ""
	switch contentType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		// Explicitly check RIFF/WEBP signatures (Go's image stdlib lacks WebP decode).
		if n < 12 || string(head[0:4]) != "RIFF" || string(head[8:12]) != "WEBP" {
			file.Close()
			return "", errors.New("invalid WebP signature")
		}
		extension = ".webp"
	default:
		file.Close()
		return "", errors.New("unsupported image type")
	}
	if extension != ".webp" {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			return "", err
		}
		cfg, _, err := image.DecodeConfig(file)
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 12000 || cfg.Height > 12000 {
			file.Close()
			return "", errors.New("invalid image dimensions")
		}
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(bytes[:]) + extension
	if err := os.Rename(tempName, filepath.Join(folder, name)); err != nil {
		return "", err
	}
	return "/static/uploads/dorms/" + name, nil
}

// Renders the existing templates/landlord_add.html form.

func renderLandlordAdd(w http.ResponseWriter, errorMessage string, csrfToken string) {

	tmpl, err := template.ParseFiles("templates/landlord_add.html")

	if err != nil {

		log.Println("Landlord Add Template Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธเธญเธฃเนเธกเนเธ”เน", http.StatusInternalServerError)

		return

	}

	data := struct {
		Error string

		CSRFToken string
	}{

		Error: errorMessage,

		CSRFToken: csrfToken,
	}

	if err := tmpl.Execute(w, data); err != nil {

		log.Println("Landlord Add Render Error:", err)

	}

}

// =========================================================

// LANDLORD - ADD ROOM TO OWN DORM

// =========================================================

func landlordRoomAddHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/landlord/rooms/add" {

		http.NotFound(w, r)

		return

	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	session, err := store.Get(r, "dormhub-session")

	if err != nil {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	var userID int

	switch v := session.Values["user_id"].(type) {

	case int:

		userID = v

	case int64:

		userID = int(v)

	default:

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if userID <= 0 {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	var role string

	err = db.QueryRowContext(r.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)

	if err == sql.ErrNoRows {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if err != nil {

		log.Println("Room Add Role Query Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(w, "403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)

		return

	}

	var dormID int

	if r.Method == http.MethodGet {

		dormID, err = strconv.Atoi(r.URL.Query().Get("dorm_id"))

	} else {

		r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

		if err := r.ParseForm(); err != nil {

			http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธเธซเธฃเธทเธญเธกเธตเธเธเธฒเธ”เนเธซเธเนเน€เธเธดเธเนเธ", http.StatusBadRequest)

			return

		}

		dormID, err = strconv.Atoi(r.PostFormValue("dorm_id"))

	}

	if err != nil || dormID <= 0 {

		http.Error(w, "เธเธฃเธธเธ“เธฒเธฃเธฐเธเธธเธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธซเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)

		return

	}

	// Never trust the hidden dorm_id alone. Query using authenticated owner_id.

	var dormName string

	err = db.QueryRowContext(r.Context(),

		"SELECT name FROM dormitories WHERE id = ? AND owner_id = ?", dormID, userID,
	).Scan(&dormName)

	if err == sql.ErrNoRows {

		http.Error(w, "เนเธกเนเธเธเธซเธญเธเธฑเธ เธซเธฃเธทเธญเธเธธเธ“เนเธกเนเธกเธตเธชเธดเธ—เธเธดเนเธเธฑเธ”เธเธฒเธฃเธซเธญเธเธฑเธเธเธตเน", http.StatusForbidden)

		return

	}

	if err != nil {

		log.Println("Room Add Dorm Query Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	if r.Method == http.MethodGet {

		tokenBytes := make([]byte, 32)

		if _, err := rand.Read(tokenBytes); err != nil {

			log.Println("Room Add CSRF Error:", err)

			http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

			return

		}

		csrfToken := hex.EncodeToString(tokenBytes)

		session.Values["landlord_room_add_csrf"] = csrfToken

		if err := session.Save(r, w); err != nil {

			log.Println("Room Add Session Error:", err)

			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)

			return

		}

		renderLandlordRoomAdd(w, dormID, dormName, csrfToken, "")

		return

	}

	savedToken, ok := session.Values["landlord_room_add_csrf"].(string)

	submittedToken := r.PostFormValue("csrf_token")

	if !ok || savedToken == "" || subtle.ConstantTimeCompare([]byte(savedToken), []byte(submittedToken)) != 1 {

		http.Error(w, "403 Forbidden: CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธเธญเธฃเนเธกเนเธซเธกเน", http.StatusForbidden)

		return

	}

	roomNumber := strings.TrimSpace(r.PostFormValue("room_number"))

	roomType := r.PostFormValue("room_type")

	status := r.PostFormValue("status")

	priceText := strings.TrimSpace(r.PostFormValue("price"))

	description := strings.TrimSpace(r.PostFormValue("description"))

	showError := func(message string) {

		renderLandlordRoomAdd(w, dormID, dormName, savedToken, message)

	}

	if roomNumber == "" || utf8.RuneCountInString(roomNumber) > 20 {

		showError("เธเธฃเธธเธ“เธฒเธเธฃเธญเธเน€เธฅเธเธซเนเธญเธเนเธกเนเน€เธเธดเธ 20 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ")

		return

	}

	if roomType != "air" && roomType != "fan" {

		showError("เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธเธฃเธฐเน€เธ เธ—เธซเนเธญเธเนเธซเนเธ–เธนเธเธ•เนเธญเธ")

		return

	}

	if status != "available" && status != "occupied" && status != "maintenance" {

		showError("เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธชเธ–เธฒเธเธฐเธซเนเธญเธเนเธซเนเธ–เธนเธเธ•เนเธญเธ")

		return

	}

	price, priceErr := strconv.ParseFloat(priceText, 64)

	if priceErr != nil || math.IsNaN(price) || math.IsInf(price, 0) ||

		price <= 0 || price > 99999999.99 || math.Abs(price*100-math.Round(price*100)) > 0.00001 {

		showError("เธเธฃเธธเธ“เธฒเธเธฃเธญเธเธฃเธฒเธเธฒเนเธซเนเธ–เธนเธเธ•เนเธญเธ (เธกเธฒเธเธเธงเนเธฒ 0 เนเธฅเธฐเธ—เธจเธเธดเธขเธกเนเธกเนเน€เธเธดเธ 2 เธ•เธณเนเธซเธเนเธ)")

		return

	}

	if utf8.RuneCountInString(description) > 5000 {

		showError("เธฃเธฒเธขเธฅเธฐเน€เธญเธตเธขเธ”เธซเนเธญเธเธขเธฒเธงเน€เธเธดเธเธเธณเธซเธเธ”")

		return

	}

	var existingCount int

	err = db.QueryRowContext(r.Context(),

		"SELECT COUNT(*) FROM rooms WHERE dormitory_id = ? AND room_number = ?",

		dormID, roomNumber,
	).Scan(&existingCount)

	if err != nil {

		log.Println("Room Add Duplicate Query Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

		return

	}

	if existingCount > 0 {

		showError("เน€เธฅเธเธซเนเธญเธเธเธตเนเธกเธตเธญเธขเธนเนเนเธเธซเธญเธเธฑเธเนเธฅเนเธง")

		return

	}

	// Recheck ownership directly in the INSERT; never accept owner_id from HTML.

	result, err := db.ExecContext(r.Context(), `

        INSERT INTO rooms (dormitory_id, room_number, room_type, price, status, description)

        SELECT d.id, ?, ?, ?, ?, ?

        FROM dormitories d

        WHERE d.id = ? AND d.owner_id = ?

    `, roomNumber, roomType, priceText, status, description, dormID, userID)

	if err != nil {

		log.Println("Room Add Insert Error:", err)

		showError("เธเธฑเธเธ—เธถเธเธซเนเธญเธเธเธฑเธเนเธกเนเธชเธณเน€เธฃเนเธ เธซเธฃเธทเธญเน€เธฅเธเธซเนเธญเธเธเนเธณ เธเธฃเธธเธ“เธฒเธฅเธญเธเธญเธตเธเธเธฃเธฑเนเธ")

		return

	}

	affected, err := result.RowsAffected()

	if err != nil || affected != 1 {

		http.Error(w, "เนเธกเนเธกเธตเธชเธดเธ—เธเธดเนเน€เธเธดเนเธกเธซเนเธญเธเธเธฑเธเนเธเธซเธญเธเธตเน", http.StatusForbidden)

		return

	}

	delete(session.Values, "landlord_room_add_csrf")

	if err := session.Save(r, w); err != nil {

		log.Println("Room Add CSRF Clear Error:", err)

	}

	log.Printf("Landlord %d added room %s to dorm %d", userID, roomNumber, dormID)

	http.Redirect(w, r, "/landlord?room_added=1", http.StatusSeeOther)

}

func renderLandlordRoomAdd(w http.ResponseWriter, dormID int, dormName, csrfToken, errorMessage string) {

	tmpl, err := template.ParseFiles("templates/landlord_room_add.html")

	if err != nil {

		log.Println("Room Add Template Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเน€เธเธดเนเธกเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	data := struct {
		DormID int

		DormName string

		CSRFToken string

		Error string
	}{

		DormID: dormID, DormName: dormName,

		CSRFToken: csrfToken, Error: errorMessage,
	}

	if err := tmpl.Execute(w, data); err != nil {

		log.Println("Room Add Template Render Error:", err)

	}

}

// =========================================================

// LANDLORD - ROOM LIST

// =========================================================

func landlordRoomsHandler(w http.ResponseWriter, r *http.Request) {

	// เธ•เธฃเธงเธเธชเธญเธ URL

	if r.URL.Path != "/landlord/rooms" {

		http.NotFound(w, r)

		return

	}

	if r.Method != http.MethodGet {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	// =========================

	// CHECK SESSION

	// =========================

	session, err := store.Get(r, "dormhub-session")

	if err != nil {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	var userID int

	switch v := session.Values["user_id"].(type) {

	case int:

		userID = v

	case int64:

		userID = int(v)

	default:

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if userID <= 0 {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	// =========================

	// CHECK LANDLORD ROLE

	// =========================

	var role string

	err = db.QueryRowContext(

		r.Context(),

		"SELECT role FROM users WHERE id = ?",

		userID,
	).Scan(&role)

	if err == sql.ErrNoRows {

		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return

	}

	if err != nil {

		log.Println("Room List User Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(

			w,

			"403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ",

			http.StatusForbidden,
		)

		return

	}

	// =========================

	// GET DORM ID

	// =========================

	dormID, err := strconv.Atoi(r.URL.Query().Get("dorm_id"))

	if err != nil || dormID <= 0 {

		http.Error(w, "เธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)

		return

	}

	// =========================

	// VERIFY DORM OWNERSHIP

	// =========================

	var dormName string

	err = db.QueryRowContext(

		r.Context(),

		`

        SELECT name

        FROM dormitories

        WHERE id = ? AND owner_id = ?

        `,

		dormID,

		userID,
	).Scan(&dormName)

	if err == sql.ErrNoRows {

		http.NotFound(w, r)

		return

	}

	if err != nil {

		log.Println("Dorm Ownership Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	// =========================

	// DATA STRUCTURES

	// =========================

	type RoomItem struct {
		ID int

		RoomNumber string

		RoomType string

		Price float64

		Status string
	}

	type RoomListData struct {
		DormID int

		DormName string

		RoomCount int

		Rooms []RoomItem

		CSRFToken string

		Deleted bool

		DeleteError string
	}

	data := RoomListData{

		DormID: dormID,

		DormName: dormName,

		Rooms: []RoomItem{},
	}

	// =========================

	// QUERY ROOMS

	// =========================

	rows, err := db.QueryContext(

		r.Context(),

		`

        SELECT

            r.id,

            r.room_number,

            r.room_type,

            r.price,

            r.status

        FROM rooms r

        INNER JOIN dormitories d

            ON r.dormitory_id = d.id

        WHERE r.dormitory_id = ?

          AND d.owner_id = ?

        ORDER BY r.room_number ASC

        `,

		dormID,

		userID,
	)

	if err != nil {

		log.Println("Room List Query Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	for rows.Next() {

		var room RoomItem

		err := rows.Scan(

			&room.ID,

			&room.RoomNumber,

			&room.RoomType,

			&room.Price,

			&room.Status,
		)

		if err != nil {

			rows.Close()

			log.Println("Room Scan Error:", err)

			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

			return

		}

		data.Rooms = append(data.Rooms, room)

	}

	if err := rows.Err(); err != nil {

		rows.Close()

		log.Println("Room Rows Error:", err)

		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธญเนเธฒเธเธซเนเธญเธเธเธฑเธ", http.StatusInternalServerError)

		return

	}

	rows.Close()

	data.RoomCount = len(data.Rooms)

	// Create a CSRF token for the POST delete forms on this page.
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		log.Println("Room List CSRF Generate Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเนเธเธเธเธญเธฃเนเธกเนเธ”เน", http.StatusInternalServerError)
		return
	}
	data.CSRFToken = hex.EncodeToString(csrfBytes)
	session.Values["landlord_room_delete_csrf"] = data.CSRFToken
	if err := session.Save(r, w); err != nil {
		log.Println("Room List CSRF Save Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)
		return
	}
	data.Deleted = r.URL.Query().Get("deleted") == "1"
	if r.URL.Query().Get("delete_error") == "has_requests" {
		data.DeleteError = "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธฅเธเธซเนเธญเธเธเธตเนเนเธ”เน เน€เธเธฃเธฒเธฐเธกเธตเธเธฃเธฐเธงเธฑเธ•เธดเธเธณเธเธญเน€เธเนเธฒเธญเธขเธนเน"
	}

	// =========================

	// LOAD TEMPLATE

	// =========================

	tmpl, err := template.ParseFiles(

		"templates/landlord_rooms.html",
	)

	if err != nil {

		log.Println("Room List Template Error:", err)

		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธเธฑเธ”เธเธฒเธฃเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)

		return

	}

	// =========================

	// RENDER

	// =========================

	if err := tmpl.Execute(w, data); err != nil {

		log.Println("Room List Render Error:", err)

	}

}

// =========================================================
// LANDLORD - EDIT ROOM (OWNER ONLY)
// =========================================================

type landlordRoomEditData struct {
	Room      landlordEditableRoom
	CSRFToken string
	Error     string
}

type landlordEditableRoom struct {
	ID          int
	DormID      int
	DormName    string
	RoomNumber  string
	RoomType    string
	Price       float64
	Status      string
	Description string
}

func landlordRoomEditHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/landlord/rooms/edit" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var userID int
	switch v := session.Values["user_id"].(type) {
	case int:
		userID = v
	case int64:
		userID = int(v)
	}
	if userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// The role is verified against MySQL on EVERY request, not just the session.
	var role string
	err = db.QueryRowContext(r.Context(),
		"SELECT role FROM users WHERE id = ?", userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Println("Room Edit Role Error:", err)
		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธ•เธฃเธงเธเธชเธญเธเธชเธดเธ—เธเธดเน", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเธซเนเธญเธเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	// Ownership check also supplies the original room info for the form.
	var room landlordEditableRoom
	err = db.QueryRowContext(r.Context(), `
        SELECT r.id, r.dormitory_id, d.name, r.room_number,
               r.room_type, r.price, r.status, COALESCE(r.description, '')
        FROM rooms r
        JOIN dormitories d ON d.id = r.dormitory_id
        WHERE r.id = ? AND d.owner_id = ?
    `, roomID, userID).Scan(
		&room.ID, &room.DormID, &room.DormName, &room.RoomNumber,
		&room.RoomType, &room.Price, &room.Status, &room.Description,
	)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Room Edit Fetch Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Println("Room Edit CSRF Generation Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธเธดเธ”เธเธญเธฃเนเธกเนเธเนเนเธเนเธ”เน", http.StatusInternalServerError)
			return
		}
		csrfToken := hex.EncodeToString(tokenBytes)
		session.Values["landlord_room_edit_csrf"] = csrfToken
		if err := session.Save(r, w); err != nil {
			log.Println("Room Edit CSRF Save Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)
			return
		}
		renderLandlordRoomEdit(w, landlordRoomEditData{Room: room, CSRFToken: csrfToken})
		return
	}

	// POST: limit payload size and validate the CSRF token.
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธเธซเธฃเธทเธญเธกเธตเธเธเธฒเธ”เนเธซเธเนเน€เธเธดเธเนเธ", http.StatusBadRequest)
		return
	}
	savedToken, ok := session.Values["landlord_room_edit_csrf"].(string)
	submittedToken := r.PostFormValue("csrf_token")
	if !ok || savedToken == "" ||
		subtle.ConstantTimeCompare([]byte(savedToken), []byte(submittedToken)) != 1 {
		http.Error(w, "403 Forbidden: CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธเธญเธฃเนเธกเนเธซเธกเน", http.StatusForbidden)
		return
	}

	room.RoomNumber = strings.TrimSpace(r.PostFormValue("room_number"))
	room.RoomType = r.PostFormValue("room_type")
	room.Status = r.PostFormValue("status")
	room.Description = strings.TrimSpace(r.PostFormValue("description"))
	priceText := strings.TrimSpace(r.PostFormValue("price"))
	price, priceErr := strconv.ParseFloat(priceText, 64)
	room.Price = price

	showError := func(message string) {
		renderLandlordRoomEdit(w, landlordRoomEditData{
			Room: room, CSRFToken: savedToken, Error: message,
		})
	}

	if room.RoomNumber == "" || utf8.RuneCountInString(room.RoomNumber) > 20 {
		showError("เธเธฃเธธเธ“เธฒเธเธฃเธญเธเน€เธฅเธเธซเนเธญเธเนเธกเนเน€เธเธดเธ 20 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ")
		return
	}
	if room.RoomType != "air" && room.RoomType != "fan" {
		showError("เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธเธฃเธฐเน€เธ เธ—เธซเนเธญเธเนเธซเนเธ–เธนเธเธ•เนเธญเธ")
		return
	}
	if room.Status != "available" && room.Status != "occupied" && room.Status != "maintenance" {
		showError("เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธชเธ–เธฒเธเธฐเธซเนเธญเธเนเธซเนเธ–เธนเธเธ•เนเธญเธ")
		return
	}
	if priceErr != nil || math.IsNaN(price) || math.IsInf(price, 0) ||
		price <= 0 || price > 99999999.99 ||
		math.Abs(price*100-math.Round(price*100)) > 0.00001 {
		showError("เธเธฃเธธเธ“เธฒเธเธฃเธญเธเธฃเธฒเธเธฒเนเธซเนเธ–เธนเธเธ•เนเธญเธ (เธกเธฒเธเธเธงเนเธฒ 0 เนเธฅเธฐเธ—เธจเธเธดเธขเธกเนเธกเนเน€เธเธดเธ 2 เธ•เธณเนเธซเธเนเธ)")
		return
	}
	if utf8.RuneCountInString(room.Description) > 5000 {
		showError("เธฃเธฒเธขเธฅเธฐเน€เธญเธตเธขเธ”เธซเนเธญเธเธขเธฒเธงเน€เธเธดเธเธเธณเธซเธเธ”")
		return
	}

	// Duplicate numbers are checked within this dorm, excluding this room.
	// A UNIQUE(dormitory_id, room_number) index also protects concurrent edits.
	var duplicates int
	err = db.QueryRowContext(r.Context(), `
        SELECT COUNT(*) FROM rooms
        WHERE dormitory_id = ? AND room_number = ? AND id <> ?
    `, room.DormID, room.RoomNumber, roomID).Scan(&duplicates)
	if err != nil {
		log.Println("Room Edit Duplicate Check Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเน€เธฅเธเธซเนเธญเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if duplicates != 0 {
		showError("เน€เธฅเธเธซเนเธญเธเธเธตเนเธกเธตเธญเธขเธนเนเนเธฅเนเธงเนเธเธซเธญเธเธฑเธเน€เธ”เธตเธขเธงเธเธฑเธ")
		return
	}

	// Re-check ownership in the UPDATE itself to prevent cross-owner edits.
	result, err := db.ExecContext(r.Context(), `
        UPDATE rooms r
        INNER JOIN dormitories d ON d.id = r.dormitory_id
        SET r.room_number = ?, r.room_type = ?, r.price = ?,
            r.status = ?, r.description = ?
        WHERE r.id = ? AND d.owner_id = ?
    `, room.RoomNumber, room.RoomType, priceText, room.Status,
		room.Description, roomID, userID)
	if err != nil {
		log.Println("Room Edit Update Error:", err)
		showError("เธเธฑเธเธ—เธถเธเนเธกเนเธชเธณเน€เธฃเนเธ เธญเธฒเธเธกเธตเน€เธฅเธเธซเนเธญเธเธเนเธณ เธเธฃเธธเธ“เธฒเธ•เธฃเธงเธเธชเธญเธเธเนเธญเธกเธนเธฅเนเธฅเนเธงเธฅเธญเธเธญเธตเธเธเธฃเธฑเนเธ")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("Room Edit RowsAffected Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธฅเธเธฒเธฃเธเธฑเธเธ—เธถเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	// MySQL returns 0 for unchanged values; verify ownership still exists.
	if affected == 0 {
		var stillOwned int
		err = db.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM rooms r JOIN dormitories d ON d.id = r.dormitory_id
			WHERE r.id = ? AND d.owner_id = ?
		`, roomID, userID).Scan(&stillOwned)
		if err != nil {
			log.Println("Room Edit Verify Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธฅเธเธฒเธฃเธเธฑเธเธ—เธถเธเนเธ”เน", http.StatusInternalServerError)
			return
		}
		if stillOwned != 1 {
			http.NotFound(w, r)
			return
		}
	}

	delete(session.Values, "landlord_room_edit_csrf")
	if err := session.Save(r, w); err != nil {
		log.Println("Room Edit CSRF Clear Error:", err)
	}
	http.Redirect(w, r,
		"/landlord/rooms?dorm_id="+strconv.Itoa(room.DormID)+"&updated=1",
		http.StatusSeeOther)
}

func renderLandlordRoomEdit(w http.ResponseWriter, data landlordRoomEditData) {
	tmpl, err := template.ParseFiles("templates/landlord_room_edit.html")
	if err != nil {
		log.Println("Room Edit Template Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเนเธเนเนเธเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("Room Edit Render Error:", err)
	}
}

// =========================================================
// LANDLORD - DELETE ROOM (OWNER ONLY, POST + CSRF)
// =========================================================

func landlordRoomDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/landlord/rooms/delete" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var userID int
	switch v := session.Values["user_id"].(type) {
	case int:
		userID = v
	case int64:
		userID = int(v)
	}
	if userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var role string
	err = db.QueryRowContext(r.Context(),
		"SELECT role FROM users WHERE id = ?", userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Println("Room Delete Role Error:", err)
		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เธเธญเธเธฃเธฐเธเธ", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	csrf, ok := session.Values["landlord_room_delete_csrf"].(string)
	provided := r.PostFormValue("csrf_token")
	if !ok || csrf == "" || subtle.ConstantTimeCompare([]byte(csrf), []byte(provided)) != 1 {
		http.Error(w, "403 Forbidden: CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธฃเธฒเธขเธเธฒเธฃเธซเนเธญเธเธเธฑเธเนเธซเธกเน", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.PostFormValue("room_id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเธซเนเธญเธเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	// Lock the room row while checking for rental requests. This protects
	// existing request history from the rooms -> rental_requests CASCADE FK.
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Room Delete BeginTx Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธฃเธดเนเธกเธฅเธเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var dormID int
	err = tx.QueryRowContext(r.Context(), `
		SELECT r.dormitory_id
		FROM rooms r
		INNER JOIN dormitories d ON d.id = r.dormitory_id
		WHERE r.id = ? AND d.owner_id = ?
		FOR UPDATE
	`, roomID, userID).Scan(&dormID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Room Delete Ownership Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธชเธดเธ—เธเธดเนเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}

	var requestCount int
	err = tx.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM rental_requests WHERE room_id = ?", roomID,
	).Scan(&requestCount)
	if err != nil {
		log.Println("Room Delete Request Check Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธฃเธฐเธงเธฑเธ•เธดเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if requestCount > 0 {
		// The room is retained. Use a normal redirect back to the room list.
		http.Redirect(w, r, "/landlord/rooms?dorm_id="+strconv.Itoa(dormID)+
			"&delete_error=has_requests", http.StatusSeeOther)
		return
	}

	result, err := tx.ExecContext(r.Context(),
		"DELETE FROM rooms WHERE id = ? AND dormitory_id = ?", roomID, dormID)
	if err != nil {
		log.Println("Room Delete SQL Error:", err)
		http.Error(w, "เธฅเธเธซเนเธญเธเธเธฑเธเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		log.Println("Room Delete RowsAffected Error:", err, "rows:", count)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธขเธทเธเธขเธฑเธเธเธฒเธฃเธฅเธเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Println("Room Delete Commit Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธเธเธฒเธฃเธฅเธเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}

	delete(session.Values, "landlord_room_delete_csrf")
	if err := session.Save(r, w); err != nil {
		log.Println("Room Delete CSRF Clear Error:", err)
	}
	log.Printf("Landlord %d deleted room %d from dorm %d", userID, roomID, dormID)
	http.Redirect(w, r, "/landlord/rooms?dorm_id="+strconv.Itoa(dormID)+"&deleted=1", http.StatusSeeOther)
}

// =========================================================
// PUBLIC - DORM DETAIL
// =========================================================

type DormDetailInfo struct {
	ID          int
	Name        string
	Address     string
	Description string
	Image       string
}

type DormDetailRoom struct {
	ID          int
	RoomNumber  string
	RoomType    string
	Price       float64
	Description string
}

type DormDetailPageData struct {
	Dorm          DormDetailInfo
	Rooms         []DormDetailRoom
	RoomCount     int
	MinPrice      float64
	HasRooms      bool
	IsLoggedIn    bool
	Name          string
	Role          string
	Reviews       []DormReviewItem
	ReviewCount   int
	AverageRating float64
	IsFavorite    bool
	HasMyReview   bool
	MyRating      int
	MyComment     string
	CSRFToken     string
	Notice        string
}

// Everyone can view public dorm listings. Only rooms marked available appear.
func dormDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/dorm" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	dormID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || dormID <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	page := DormDetailPageData{Rooms: []DormDetailRoom{}, Reviews: []DormReviewItem{}}
	err = db.QueryRowContext(r.Context(), `
        SELECT id, name, address, COALESCE(description, ''), COALESCE(image, '')
        FROM dormitories
        WHERE id = ?
    `, dormID).Scan(
		&page.Dorm.ID, &page.Dorm.Name, &page.Dorm.Address,
		&page.Dorm.Description, &page.Dorm.Image,
	)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Dorm Detail Query Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}

	// The public page must never show occupied or under-maintenance rooms.
	rows, err := db.QueryContext(r.Context(), `
        SELECT id, room_number, room_type, price, COALESCE(description, '')
        FROM rooms
        WHERE dormitory_id = ? AND status = 'available'
        ORDER BY price ASC, room_number ASC
        LIMIT 200
    `, dormID)
	if err != nil {
		log.Println("Dorm Detail Rooms Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var room DormDetailRoom
		if err = rows.Scan(&room.ID, &room.RoomNumber, &room.RoomType,
			&room.Price, &room.Description); err != nil {
			rows.Close()
			log.Println("Dorm Detail Room Scan Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
			return
		}
		if !page.HasRooms || room.Price < page.MinPrice {
			page.MinPrice = room.Price
		}
		page.HasRooms = true
		page.Rooms = append(page.Rooms, room)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		log.Println("Dorm Detail Room Rows Error:", err)
		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธญเนเธฒเธเธซเนเธญเธเธเธฑเธ", http.StatusInternalServerError)
		return
	}
	rows.Close()
	page.RoomCount = len(page.Rooms)

	// Session is optional because the detail page is public.
	session, err := store.Get(r, "dormhub-session")
	if err == nil {
		switch userID := session.Values["user_id"].(type) {
		case int:
			page.IsLoggedIn = userID > 0
		case int64:
			page.IsLoggedIn = userID > 0
		}
		if page.IsLoggedIn {
			page.Name, _ = session.Values["name"].(string)
			page.Role, _ = session.Values["role"].(string)
		}
	}

	// Step 12: show public rating summary and latest reviews.
	err = db.QueryRowContext(r.Context(), `
        SELECT COUNT(*), COALESCE(AVG(rating), 0)
        FROM reviews WHERE dormitory_id = ?
    `, dormID).Scan(&page.ReviewCount, &page.AverageRating)
	if err != nil {
		log.Println("Dorm Reviews Summary Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเธฐเนเธเธเธฃเธตเธงเธดเธงเนเธ”เน", http.StatusInternalServerError)
		return
	}
	reviewRows, err := db.QueryContext(r.Context(), `
        SELECT rev.id, u.name, rev.rating, COALESCE(rev.comment,''), rev.created_at
        FROM reviews rev JOIN users u ON u.id = rev.user_id
        WHERE rev.dormitory_id = ? ORDER BY rev.created_at DESC, rev.id DESC LIMIT 100
    `, dormID)
	if err != nil {
		log.Println("Dorm Reviews Query Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธตเธงเธดเธงเนเธ”เน", http.StatusInternalServerError)
		return
	}
	for reviewRows.Next() {
		var item DormReviewItem
		var created time.Time
		if err := reviewRows.Scan(&item.ID, &item.Name, &item.Rating, &item.Comment, &created); err != nil {
			reviewRows.Close()
			log.Println("Dorm Reviews Scan Error:", err)
			http.Error(w, "เธญเนเธฒเธเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
			return
		}
		item.CreatedAt = created.Format("02/01/2006 15:04")
		page.Reviews = append(page.Reviews, item)
	}
	if err := reviewRows.Err(); err != nil {
		reviewRows.Close()
		log.Println("Dorm Reviews Rows Error:", err)
		http.Error(w, "เธญเนเธฒเธเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	reviewRows.Close()

	// Only tenant accounts may save reviews / favorites.
	if page.IsLoggedIn && page.Role == "user" {
		var currentUserID int
		switch val := session.Values["user_id"].(type) {
		case int:
			currentUserID = val
		case int64:
			currentUserID = int(val)
		}
		if currentUserID > 0 {
			page.CSRFToken, err = step12CSRFToken(w, r, session)
			if err != nil {
				log.Println("Dorm Detail CSRF Error:", err)
				http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธเธดเธ”เนเธเธเธเธญเธฃเนเธกเนเธ”เน", http.StatusInternalServerError)
				return
			}
			var favoriteCount int
			if err := db.QueryRowContext(r.Context(),
				"SELECT COUNT(*) FROM favorites WHERE user_id = ? AND dormitory_id = ?",
				currentUserID, dormID).Scan(&favoriteCount); err != nil {
				log.Println("Dorm Favorite Status Error:", err)
				http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธ”เน", http.StatusInternalServerError)
				return
			}
			page.IsFavorite = favoriteCount > 0
			err = db.QueryRowContext(r.Context(), `
                SELECT rating, COALESCE(comment, '')
                FROM reviews WHERE user_id = ? AND dormitory_id = ?
                LIMIT 1
            `, currentUserID, dormID).Scan(&page.MyRating, &page.MyComment)
			if err != nil && err != sql.ErrNoRows {
				log.Println("Dorm My Review Error:", err)
				http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธตเธงเธดเธงเธเธญเธเธเธธเธ“เนเธ”เน", http.StatusInternalServerError)
				return
			}
			page.HasMyReview = err == nil
		}
	}
	if r.URL.Query().Get("review") == "saved" {
		page.Notice = "เธเธฑเธเธ—เธถเธเธฃเธตเธงเธดเธงเธชเธณเน€เธฃเนเธเนเธฅเนเธง"
	}
	if r.URL.Query().Get("favorite") == "add" {
		page.Notice = "เน€เธเธดเนเธกเธซเธญเธเธฑเธเนเธเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธฅเนเธง"
	} else if r.URL.Query().Get("favorite") == "remove" {
		page.Notice = "เธเธณเธซเธญเธเธฑเธเธญเธญเธเธเธฒเธเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธฅเนเธง"
	}

	tmpl, err := template.ParseFiles("templates/dorm_detail.html")
	if err != nil {
		log.Println("Dorm Detail Template Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธฃเธฒเธขเธฅเธฐเน€เธญเธตเธขเธ”เธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Println("Dorm Detail Render Error:", err)
	}
}

// =========================================================
// TENANT - CREATE RENTAL REQUEST
// =========================================================

type RentalRequestPageData struct {
	DormID      int
	DormName    string
	RoomID      int
	RoomNumber  string
	RoomType    string
	Price       float64
	Description string
	Name        string
	Phone       string
	MoveInDate  string
	Message     string
	CSRFToken   string
	HasPending  bool
	Success     bool
	Error       string
	Today       string
}

func renderRentalRequest(w http.ResponseWriter, page RentalRequestPageData) {
	tmpl, err := template.ParseFiles("templates/rental_request.html")
	if err != nil {
		log.Println("Rental Request Template Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เนเธเธเธเธญเธฃเนเธกเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Println("Rental Request Render Error:", err)
	}
}

func rentalRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rental/request" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Identify user using the signed session, never from a form input.
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var userID int
	switch id := session.Values["user_id"].(type) {
	case int:
		userID = id
	case int64:
		userID = int(id)
	}
	if userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Only registered tenants can send requests (not landlord/admin).
	var userName, userPhone, userRole string
	err = db.QueryRowContext(r.Context(),
		"SELECT name, phone, role FROM users WHERE id = ?", userID,
	).Scan(&userName, &userPhone, &userRole)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Println("Rental Request User Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเนเธญเธกเธนเธฅเธเธนเนเนเธเนเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if userRole != "user" {
		http.Error(w, "เน€เธเธเธฒเธฐเธเธฑเธเธเธตเธเธนเนเน€เธเนเธฒเน€เธ—เนเธฒเธเธฑเนเธเธ—เธตเนเธชเนเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.URL.Query().Get("room_id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเธซเนเธญเธเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	page := RentalRequestPageData{
		RoomID: roomID,
		Name:   userName,
		Phone:  userPhone,
		Today:  time.Now().In(time.Local).Format("2006-01-02"),
	}
	var status string
	err = db.QueryRowContext(r.Context(), `
        SELECT d.id, d.name, r.room_number, r.room_type,
               r.price, COALESCE(r.description, ''), r.status
        FROM rooms r
        INNER JOIN dormitories d ON d.id = r.dormitory_id
        WHERE r.id = ?
    `, roomID).Scan(&page.DormID, &page.DormName, &page.RoomNumber,
		&page.RoomType, &page.Price, &page.Description, &status)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Rental Request Room Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธซเนเธญเธเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		var pendingCount int
		err = db.QueryRowContext(r.Context(), `
            SELECT COUNT(*) FROM rental_requests
            WHERE user_id = ? AND room_id = ? AND status = 'pending'
        `, userID, roomID).Scan(&pendingCount)
		if err != nil {
			log.Println("Rental Request Pending Check Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
			return
		}
		page.HasPending = pendingCount > 0
		page.Success = page.HasPending && r.URL.Query().Get("sent") == "1"
		if status != "available" && !page.HasPending {
			http.Error(w, "เธซเนเธญเธเธเธฑเธเธเธตเนเธขเธฑเธเนเธกเนเน€เธเธดเธ”เธฃเธฑเธเธเธณเธเธญเน€เธเนเธฒ", http.StatusConflict)
			return
		}
		if !page.HasPending {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				log.Println("Rental Request CSRF Error:", err)
				http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธชเธฃเนเธฒเธเธเธญเธฃเนเธก", http.StatusInternalServerError)
				return
			}
			page.CSRFToken = hex.EncodeToString(tokenBytes)
			session.Values["rental_request_csrf"] = page.CSRFToken
			session.Values["rental_request_csrf_room"] = roomID
			if err := session.Save(r, w); err != nil {
				log.Println("Rental Request Session Save Error:", err)
				http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)
				return
			}
		}
		renderRentalRequest(w, page)
		return
	}

	// POST: enforce a small request body and validate CSRF token.
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธเธซเธฃเธทเธญเธกเธตเธเธเธฒเธ”เนเธซเธเนเน€เธเธดเธเนเธ", http.StatusBadRequest)
		return
	}
	savedToken, tokenOK := session.Values["rental_request_csrf"].(string)
	tokenRoomID, roomOK := session.Values["rental_request_csrf_room"].(int)
	providedToken := r.PostFormValue("csrf_token")
	if !tokenOK || !roomOK || tokenRoomID != roomID || savedToken == "" ||
		subtle.ConstantTimeCompare([]byte(savedToken), []byte(providedToken)) != 1 {
		http.Error(w, "403 Forbidden: เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธเธญเธฃเนเธกเธเธณเธเธญเน€เธเนเธฒเนเธซเธกเน", http.StatusForbidden)
		return
	}
	page.CSRFToken = savedToken
	page.Phone = strings.TrimSpace(r.PostFormValue("phone"))
	page.MoveInDate = strings.TrimSpace(r.PostFormValue("move_in_date"))
	page.Message = strings.TrimSpace(r.PostFormValue("message"))
	formError := func(message string) {
		page.Error = message
		renderRentalRequest(w, page)
	}

	if len(page.Phone) != 10 {
		formError("เธเธฃเธธเธ“เธฒเธฃเธฐเธเธธเน€เธเธญเธฃเนเนเธ—เธฃเธจเธฑเธเธ—เน 10 เธซเธฅเธฑเธ")
		return
	}
	for _, c := range page.Phone {
		if c < '0' || c > '9' {
			formError("เน€เธเธญเธฃเนเนเธ—เธฃเธจเธฑเธเธ—เนเธ•เนเธญเธเน€เธเนเธเธ•เธฑเธงเน€เธฅเธเน€เธ—เนเธฒเธเธฑเนเธ")
			return
		}
	}
	moveIn, err := time.Parse("2006-01-02", page.MoveInDate)
	if err != nil || moveIn.Format("2006-01-02") != page.MoveInDate || page.MoveInDate < page.Today {
		formError("เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธงเธฑเธเธ—เธตเนเธขเนเธฒเธขเน€เธเนเธฒเธ•เธฑเนเธเนเธ•เนเธงเธฑเธเธเธตเนเน€เธเนเธเธ•เนเธเนเธ")
		return
	}
	if utf8.RuneCountInString(page.Message) > 2000 {
		formError("เธเนเธญเธเธงเธฒเธกเธ–เธถเธเน€เธเนเธฒเธเธญเธเธซเธญเธ•เนเธญเธเนเธกเนเน€เธเธดเธ 2,000 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ")
		return
	}

	// Lock the room row to serialize requests, room status changes and deletion.
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Rental Request BeginTx Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธฃเธดเนเธกเธเธฑเธเธ—เธถเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	var actualStatus string
	err = tx.QueryRowContext(r.Context(),
		"SELECT status FROM rooms WHERE id = ? FOR UPDATE", roomID,
	).Scan(&actualStatus)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Rental Request Lock Room Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธชเธ–เธฒเธเธฐเธซเนเธญเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if actualStatus != "available" {
		formError("เธซเนเธญเธเธเธตเนเนเธกเนเธงเนเธฒเธเนเธฅเนเธง เธเธฃเธธเธ“เธฒเน€เธฅเธทเธญเธเธซเนเธญเธเธญเธทเนเธ")
		return
	}

	var existing int
	err = tx.QueryRowContext(r.Context(), `
        SELECT COUNT(*) FROM rental_requests
        WHERE user_id = ? AND room_id = ? AND status = 'pending'
    `, userID, roomID).Scan(&existing)
	if err != nil {
		log.Println("Rental Request Duplicate Check Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธณเธเธญเน€เธเนเธฒเธเนเธณเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if existing > 0 {
		http.Redirect(w, r, "/rental/request?room_id="+strconv.Itoa(roomID), http.StatusSeeOther)
		return
	}

	_, err = tx.ExecContext(r.Context(), `
        INSERT INTO rental_requests
            (user_id, room_id, phone, move_in_date, message, status)
        VALUES (?, ?, ?, ?, ?, 'pending')
    `, userID, roomID, page.Phone, page.MoveInDate, page.Message)
	if err != nil {
		log.Println("Rental Request Insert Error:", err)
		formError("เธเธฑเธเธ—เธถเธเธเธณเธเธญเน€เธเนเธฒเนเธกเนเธชเธณเน€เธฃเนเธ เธเธฃเธธเธ“เธฒเธฅเธญเธเนเธซเธกเน")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Println("Rental Request Commit Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธขเธทเธเธขเธฑเธเธเธฒเธฃเธเธฑเธเธ—เธถเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}

	delete(session.Values, "rental_request_csrf")
	delete(session.Values, "rental_request_csrf_room")
	if err := session.Save(r, w); err != nil {
		log.Println("Rental Request Session Clear Error:", err)
	}
	log.Printf("User %d requested room %d", userID, roomID)
	http.Redirect(w, r, "/rental/request?room_id="+strconv.Itoa(roomID)+"&sent=1", http.StatusSeeOther)
}

// =========================================================
// LANDLORD - REVIEW RENTAL REQUESTS
// =========================================================

type LandlordRequestItem struct {
	ID         int
	DormName   string
	RoomNumber string
	TenantName string
	Phone      string
	MoveInDate string
	Message    string
	Status     string
	CreatedAt  string
}

type LandlordRequestsPageData struct {
	Name      string
	Requests  []LandlordRequestItem
	Pending   int
	CSRFToken string
	Result    string
}

func landlordRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/landlord/requests" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only signed-in accounts with a landlord role in the database can proceed.
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var ownerID int
	switch v := session.Values["user_id"].(type) {
	case int:
		ownerID = v
	case int64:
		ownerID = int(v)
	}
	if ownerID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var name, role string
	err = db.QueryRowContext(r.Context(),
		"SELECT name, role FROM users WHERE id = ?", ownerID,
	).Scan(&name, &role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Println("Landlord Requests Auth Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธฑเธเธเธตเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เน€เธเธเธฒเธฐเน€เธเนเธฒเธเธญเธเธซเธญเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `
            SELECT rr.id, d.name, rm.room_number, u.name,
                   rr.phone, rr.move_in_date, COALESCE(rr.message, ''),
                   rr.status, rr.created_at
            FROM rental_requests rr
            INNER JOIN rooms rm ON rm.id = rr.room_id
            INNER JOIN dormitories d ON d.id = rm.dormitory_id
            INNER JOIN users u ON u.id = rr.user_id
            WHERE d.owner_id = ?
            ORDER BY CASE WHEN rr.status = 'pending' THEN 0 ELSE 1 END,
                     rr.created_at DESC, rr.id DESC
            LIMIT 300
        `, ownerID)
		if err != nil {
			log.Println("Landlord Requests Query Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธฒเธขเธเธฒเธฃเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
			return
		}
		page := LandlordRequestsPageData{
			Name:     name,
			Requests: []LandlordRequestItem{},
			Result:   r.URL.Query().Get("result"),
		}
		for rows.Next() {
			var item LandlordRequestItem
			var moveIn, createdAt time.Time
			if err := rows.Scan(&item.ID, &item.DormName, &item.RoomNumber,
				&item.TenantName, &item.Phone, &moveIn, &item.Message,
				&item.Status, &createdAt); err != nil {
				rows.Close()
				log.Println("Landlord Requests Scan Error:", err)
				http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
				return
			}
			item.MoveInDate = moveIn.Format("02/01/2006")
			item.CreatedAt = createdAt.Format("02/01/2006 15:04")
			if item.Status == "pending" {
				page.Pending++
			}
			page.Requests = append(page.Requests, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			log.Println("Landlord Requests Rows Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธฃเธฒเธขเธเธฒเธฃเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
			return
		}
		rows.Close()

		// Generate a fresh CSRF token for the review form.
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Println("Landlord Requests CSRF Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเธเธญเธฃเนเธกเนเธ”เน", http.StatusInternalServerError)
			return
		}
		page.CSRFToken = hex.EncodeToString(tokenBytes)
		session.Values["landlord_requests_csrf"] = page.CSRFToken
		if err := session.Save(r, w); err != nil {
			log.Println("Landlord Requests Save Session Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธ Session เนเธ”เน", http.StatusInternalServerError)
			return
		}
		tmpl, err := template.ParseFiles("templates/landlord_requests.html")
		if err != nil {
			log.Println("Landlord Requests Template Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธฃเธตเธงเธดเธงเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, page); err != nil {
			log.Println("Landlord Requests Render Error:", err)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	savedToken, ok := session.Values["landlord_requests_csrf"].(string)
	submittedToken := r.PostFormValue("csrf_token")
	if !ok || savedToken == "" || subtle.ConstantTimeCompare(
		[]byte(savedToken), []byte(submittedToken),
	) != 1 {
		http.Error(w, "403 Forbidden: เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธซเธเนเธฒเธเธณเธเธญเน€เธเนเธฒเนเธซเธกเน", http.StatusForbidden)
		return
	}
	requestID, err := strconv.Atoi(r.PostFormValue("request_id"))
	if err != nil || requestID <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเธเธณเธเธญเน€เธเนเธฒเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	action := r.PostFormValue("action")
	if action != "approve" && action != "reject" {
		http.Error(w, "เธเธณเธชเธฑเนเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}

	// Look up the room ID first, then lock the room before the request row.
	// This matches the room-first lock order in rentalRequestHandler.
	var roomID int
	err = db.QueryRowContext(r.Context(),
		"SELECT room_id FROM rental_requests WHERE id = ?", requestID,
	).Scan(&roomID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Landlord Requests Lookup Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Landlord Requests BeginTx Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธฃเธดเนเธกเธเธฑเธเธ—เธถเธเธเธณเธเธญเนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var roomStatus string
	err = tx.QueryRowContext(r.Context(), `
        SELECT rm.status FROM rooms rm
        JOIN dormitories d ON d.id = rm.dormitory_id
        WHERE rm.id = ? AND d.owner_id = ?
        FOR UPDATE
    `, roomID, ownerID).Scan(&roomStatus)
	if err == sql.ErrNoRows {
		// Hide other landlords' room and request information.
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Landlord Requests Ownership Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธชเธดเธ—เธเธดเนเนเธ”เน", http.StatusInternalServerError)
		return
	}

	var requestStatus string
	var lockedRoomID int
	err = tx.QueryRowContext(r.Context(),
		"SELECT room_id, status FROM rental_requests WHERE id = ? FOR UPDATE", requestID,
	).Scan(&lockedRoomID, &requestStatus)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Println("Landlord Requests Lock Request Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธชเธ–เธฒเธเธฐเธเธณเธเธญเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if lockedRoomID != roomID || requestStatus != "pending" {
		http.Redirect(w, r, "/landlord/requests?result=changed", http.StatusSeeOther)
		return
	}

	if action == "approve" {
		if roomStatus != "available" {
			http.Redirect(w, r, "/landlord/requests?result=not_available", http.StatusSeeOther)
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			"UPDATE rental_requests SET status = 'approved' WHERE id = ? AND status = 'pending'", requestID,
		); err != nil {
			log.Println("Landlord Requests Approve Error:", err)
			http.Error(w, "เธญเธเธธเธกเธฑเธ•เธดเธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			"UPDATE rooms SET status = 'occupied' WHERE id = ? AND status = 'available'", roomID,
		); err != nil {
			log.Println("Landlord Requests Occupy Room Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เน€เธเธฅเธตเนเธขเธเธชเธ–เธฒเธเธฐเธซเนเธญเธเนเธ”เน", http.StatusInternalServerError)
			return
		}
		// Other pending requests for the same room cannot be fulfilled.
		if _, err = tx.ExecContext(r.Context(), `
            UPDATE rental_requests SET status = 'rejected'
            WHERE room_id = ? AND id <> ? AND status = 'pending'
        `, roomID, requestID); err != nil {
			log.Println("Landlord Requests Reject Others Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธดเธ”เธเธณเธเธญเธญเธทเนเธเนเธ”เน", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err = tx.ExecContext(r.Context(),
			"UPDATE rental_requests SET status = 'rejected' WHERE id = ? AND status = 'pending'", requestID,
		); err != nil {
			log.Println("Landlord Requests Reject Error:", err)
			http.Error(w, "เธเธเธดเน€เธชเธเธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Println("Landlord Requests Commit Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธขเธทเธเธขเธฑเธเธเธฒเธฃเน€เธเธฅเธตเนเธขเธเนเธเธฅเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	delete(session.Values, "landlord_requests_csrf")
	if err := session.Save(r, w); err != nil {
		log.Println("Landlord Requests Clear CSRF Error:", err)
	}
	log.Printf("Landlord %d reviewed request %d: %s", ownerID, requestID, action)
	result := "rejected"
	if action == "approve" {
		result = "approved"
	}
	http.Redirect(w, r, "/landlord/requests?result="+result, http.StatusSeeOther)
}

// =========================================================
// STEP 11: TENANT - MY RENTAL REQUESTS (READ ONLY)
// =========================================================

type MyRentalRequestItem struct {
	ID         int
	DormID     int
	DormName   string
	RoomNumber string
	RoomType   string
	Price      float64
	Phone      string
	MoveInDate string
	Message    string
	Status     string
	CreatedAt  string
}

type MyRequestsPageData struct {
	Name          string
	Requests      []MyRentalRequestItem
	PendingCount  int
	ApprovedCount int
	RejectedCount int
}

func myRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/my-requests" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Never trust a user_id sent via query string or form. Always use the
	// authenticated, signed session to find the current user's requests.
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var userID int
	switch id := session.Values["user_id"].(type) {
	case int:
		userID = id
	case int64:
		userID = int(id)
	}
	if userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Ensure the account still exists and check the actual role in MySQL.
	var name, role string
	err = db.QueryRowContext(r.Context(),
		"SELECT name, role FROM users WHERE id = ?", userID,
	).Scan(&name, &role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Println("My Requests User Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเนเธญเธกเธนเธฅเธเธนเนเนเธเนเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if role != "user" {
		http.Error(w, "403 Forbidden: เธซเธเนเธฒเธเธตเนเธชเธณเธซเธฃเธฑเธเธเธนเนเน€เธเนเธฒเธซเนเธญเธเธเธฑเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return
	}

	page := MyRequestsPageData{
		Name:     name,
		Requests: []MyRentalRequestItem{},
	}

	rows, err := db.QueryContext(r.Context(), `
        SELECT rr.id, d.id, d.name, rm.room_number, rm.room_type,
               rm.price, rr.phone, rr.move_in_date,
               COALESCE(rr.message, ''), rr.status, rr.created_at
        FROM rental_requests rr
        INNER JOIN rooms rm ON rm.id = rr.room_id
        INNER JOIN dormitories d ON d.id = rm.dormitory_id
        WHERE rr.user_id = ?
        ORDER BY rr.created_at DESC, rr.id DESC
        LIMIT 200
    `, userID)
	if err != nil {
		log.Println("My Requests Query Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var item MyRentalRequestItem
		var moveInDate, createdAt time.Time
		err = rows.Scan(&item.ID, &item.DormID, &item.DormName,
			&item.RoomNumber, &item.RoomType, &item.Price,
			&item.Phone, &moveInDate, &item.Message,
			&item.Status, &createdAt)
		if err != nil {
			log.Println("My Requests Scan Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
			return
		}
		item.MoveInDate = moveInDate.Format("02/01/2006")
		item.CreatedAt = createdAt.Format("02/01/2006 15:04")
		switch item.Status {
		case "pending":
			page.PendingCount++
		case "approved":
			page.ApprovedCount++
		case "rejected":
			page.RejectedCount++
		}
		page.Requests = append(page.Requests, item)
	}
	if err := rows.Err(); err != nil {
		log.Println("My Requests Rows Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/my_requests.html")
	if err != nil {
		log.Println("My Requests Template Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธซเธเนเธฒเธเธณเธเธญเน€เธเนเธฒเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Println("My Requests Template Execute Error:", err)
	}
}

// =========================================================
// STEP 12 - REVIEWS AND FAVORITES
// =========================================================

type DormReviewItem struct {
	ID        int
	Name      string
	Rating    int
	Comment   string
	CreatedAt string
}

type FavoriteDormItem struct {
	ID             int
	Name           string
	Address        string
	Image          string
	AvailableRooms int
	MinPrice       float64
}

type FavoritesPageData struct {
	Name      string
	CSRFToken string
	Dorms     []FavoriteDormItem
}

type MyReviewItem struct {
	DormID    int
	DormName  string
	Rating    int
	Comment   string
	CreatedAt string
}

type MyReviewsPageData struct {
	Name      string
	CSRFToken string
	Reviews   []MyReviewItem
}

// step12Tenant checks the logged-in user against the real database role.
// It never trusts a user ID, owner ID, or role sent from a form.
func step12Tenant(w http.ResponseWriter, r *http.Request) (int, string, *sessions.Session, bool) {
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	var userID int
	switch value := session.Values["user_id"].(type) {
	case int:
		userID = value
	case int64:
		userID = int(value)
	}
	if userID <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	var name, role string
	err = db.QueryRowContext(r.Context(), "SELECT name, role FROM users WHERE id = ?", userID).Scan(&name, &role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	if err != nil {
		log.Println("Step 12 User Lookup Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธเธฑเธเธเธตเธเธนเนเนเธเนเนเธ”เน", http.StatusInternalServerError)
		return 0, "", nil, false
	}
	if role != "user" {
		http.Error(w, "403 Forbidden: เธชเธณเธซเธฃเธฑเธเธเธฑเธเธเธตเธเธนเนเน€เธเนเธฒเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return 0, "", nil, false
	}
	return userID, name, session, true
}

// Uses a session-bound CSRF token for all Step 12 POST forms.
func step12CSRFToken(w http.ResponseWriter, r *http.Request, session *sessions.Session) (string, error) {
	if existing, ok := session.Values["step12_csrf"].(string); ok && existing != "" {
		return existing, nil
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	session.Values["step12_csrf"] = token
	if err := session.Save(r, w); err != nil {
		return "", err
	}
	return token, nil
}

func step12VerifyCSRF(r *http.Request, session *sessions.Session) bool {
	saved, ok := session.Values["step12_csrf"].(string)
	provided := r.PostFormValue("csrf_token")
	return ok && saved != "" && provided != "" &&
		subtle.ConstantTimeCompare([]byte(saved), []byte(provided)) == 1
}

func step12PostForm(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "เธเนเธญเธกเธนเธฅเนเธเธเธเธญเธฃเนเธกเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return false
	}
	return true
}

func step12DormID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PostFormValue("dorm_id"))
	return id, err == nil && id > 0
}

// POST /favorites/toggle: explicit actions make retries predictable.
func favoriteToggleHandler(w http.ResponseWriter, r *http.Request) {
	if !step12PostForm(w, r) {
		return
	}
	userID, _, session, ok := step12Tenant(w, r)
	if !ok {
		return
	}
	if !step12VerifyCSRF(r, session) {
		http.Error(w, "403 Forbidden: CSRF token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธซเธเนเธฒเนเธซเธกเน", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "เธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	var exists int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM dormitories WHERE id = ?", dormID).Scan(&exists); err != nil {
		log.Println("Favorite Dorm Check Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธ•เธฃเธงเธเธชเธญเธเธซเธญเธเธฑเธเนเธ”เน", http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		http.NotFound(w, r)
		return
	}
	action := r.PostFormValue("action")
	var err error
	switch action {
	case "add":
		_, err = db.ExecContext(r.Context(),
			"INSERT IGNORE INTO favorites (user_id, dormitory_id) VALUES (?, ?)", userID, dormID)
	case "remove":
		_, err = db.ExecContext(r.Context(),
			"DELETE FROM favorites WHERE user_id = ? AND dormitory_id = ?", userID, dormID)
	default:
		http.Error(w, "เธเธณเธชเธฑเนเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println("Favorite Change Error:", err)
		http.Error(w, "เธเธฑเธเธ—เธถเธเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	if r.PostFormValue("from") == "favorites" {
		http.Redirect(w, r, "/favorites", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dorm?id="+strconv.Itoa(dormID)+"&favorite="+action, http.StatusSeeOther)
}

// POST /reviews/save: requires UNIQUE(user_id,dormitory_id) from step12_migration.sql.
// The same tenant can edit their review, but cannot create a second review for a dorm.
func reviewSaveHandler(w http.ResponseWriter, r *http.Request) {
	if !step12PostForm(w, r) {
		return
	}
	userID, _, session, ok := step12Tenant(w, r)
	if !ok {
		return
	}
	if !step12VerifyCSRF(r, session) {
		http.Error(w, "403 Forbidden: CSRF token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธซเธเนเธฒเนเธซเธกเน", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "เธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	rating, err := strconv.Atoi(r.PostFormValue("rating"))
	if err != nil || rating < 1 || rating > 5 {
		http.Error(w, "เธเธฃเธธเธ“เธฒเนเธซเนเธเธฐเนเธเธเธ•เธฑเนเธเนเธ•เน 1 เธ–เธถเธ 5 เธ”เธฒเธง", http.StatusBadRequest)
		return
	}
	comment := strings.TrimSpace(r.PostFormValue("comment"))
	if utf8.RuneCountInString(comment) > 1000 {
		http.Error(w, "เธฃเธตเธงเธดเธงเธ•เนเธญเธเนเธกเนเน€เธเธดเธ 1000 เธ•เธฑเธงเธญเธฑเธเธฉเธฃ", http.StatusBadRequest)
		return
	}
	var exists int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM dormitories WHERE id = ?", dormID).Scan(&exists); err != nil {
		log.Println("Review Dorm Check Error:", err)
		http.Error(w, "เธ•เธฃเธงเธเธชเธญเธเธซเธญเธเธฑเธเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		http.NotFound(w, r)
		return
	}
	_, err = db.ExecContext(r.Context(), `
        INSERT INTO reviews (user_id, dormitory_id, rating, comment)
        VALUES (?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE rating = VALUES(rating), comment = VALUES(comment)
    `, userID, dormID, rating, comment)
	if err != nil {
		log.Println("Review Save Error:", err)
		http.Error(w, "เธเธฑเธเธ—เธถเธเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ เธเธฃเธธเธ“เธฒเธ•เธฃเธงเธเธงเนเธฒเธ•เธดเธ”เธ•เธฑเนเธ SQL เธเธญเธ Step 12 เนเธฅเนเธง", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dorm?id="+strconv.Itoa(dormID)+"&review=saved#reviews", http.StatusSeeOther)
}

func reviewDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if !step12PostForm(w, r) {
		return
	}
	userID, _, session, ok := step12Tenant(w, r)
	if !ok {
		return
	}
	if !step12VerifyCSRF(r, session) {
		http.Error(w, "403 Forbidden: CSRF token เนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "เธฃเธซเธฑเธชเธซเธญเธเธฑเธเนเธกเนเธ–เธนเธเธ•เนเธญเธ", http.StatusBadRequest)
		return
	}
	if _, err := db.ExecContext(r.Context(),
		"DELETE FROM reviews WHERE user_id = ? AND dormitory_id = ?", userID, dormID); err != nil {
		log.Println("Review Delete Error:", err)
		http.Error(w, "เธฅเธเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/my-reviews?deleted=1", http.StatusSeeOther)
}

func favoritesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/favorites" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, name, session, ok := step12Tenant(w, r)
	if !ok {
		return
	}
	token, err := step12CSRFToken(w, r, session)
	if err != nil {
		log.Println("Favorites CSRF Error:", err)
		http.Error(w, "เธชเธฃเนเธฒเธเธเธญเธฃเนเธกเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	page := FavoritesPageData{Name: name, CSRFToken: token, Dorms: []FavoriteDormItem{}}
	rows, err := db.QueryContext(r.Context(), `
        SELECT d.id, d.name, d.address, COALESCE(d.image, ''),
               COUNT(r.id), COALESCE(MIN(r.price), 0)
        FROM favorites f
        JOIN dormitories d ON d.id = f.dormitory_id
        LEFT JOIN rooms r ON r.dormitory_id = d.id AND r.status = 'available'
        WHERE f.user_id = ?
        GROUP BY d.id, d.name, d.address, d.image, f.created_at
        ORDER BY f.created_at DESC, d.id DESC
        LIMIT 100
    `, userID)
	if err != nil {
		log.Println("Favorites Query Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item FavoriteDormItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Address, &item.Image,
			&item.AvailableRooms, &item.MinPrice); err != nil {
			log.Println("Favorites Scan Error:", err)
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธ”เน", http.StatusInternalServerError)
			return
		}
		page.Dorms = append(page.Dorms, item)
	}
	if err := rows.Err(); err != nil {
		log.Println("Favorites Rows Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเนเธฒเธเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธ”เน", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.ParseFiles("templates/favorites.html")
	if err != nil {
		log.Println("Favorites Template Error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธซเธเนเธฒเธฃเธฒเธขเธเธฒเธฃเนเธเธฃเธ”เนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Println("Favorites Render Error:", err)
	}
}

func myReviewsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/my-reviews" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, name, session, ok := step12Tenant(w, r)
	if !ok {
		return
	}
	token, err := step12CSRFToken(w, r, session)
	if err != nil {
		log.Println("My Reviews CSRF Error:", err)
		http.Error(w, "เธชเธฃเนเธฒเธเธเธญเธฃเนเธกเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	page := MyReviewsPageData{Name: name, CSRFToken: token, Reviews: []MyReviewItem{}}
	rows, err := db.QueryContext(r.Context(), `
        SELECT r.dormitory_id, d.name, r.rating, COALESCE(r.comment,''), r.created_at
        FROM reviews r JOIN dormitories d ON d.id = r.dormitory_id
        WHERE r.user_id = ?
        ORDER BY r.created_at DESC, r.id DESC LIMIT 100
    `, userID)
	if err != nil {
		log.Println("My Reviews Query Error:", err)
		http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เนเธซเธฅเธ”เธฃเธตเธงเธดเธงเนเธ”เน", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item MyReviewItem
		var created time.Time
		if err := rows.Scan(&item.DormID, &item.DormName, &item.Rating, &item.Comment, &created); err != nil {
			log.Println("My Reviews Scan Error:", err)
			http.Error(w, "เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
			return
		}
		item.CreatedAt = created.Format("02/01/2006 15:04")
		page.Reviews = append(page.Reviews, item)
	}
	if err := rows.Err(); err != nil {
		log.Println("My Reviews Rows Error:", err)
		http.Error(w, "เธญเนเธฒเธเธเนเธญเธกเธนเธฅเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.ParseFiles("templates/my_reviews.html")
	if err != nil {
		log.Println("My Reviews Template Error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธซเธเนเธฒเธฃเธตเธงเธดเธงเนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, page); err != nil {
		log.Println("My Reviews Render Error:", err)
	}
}

// =========================================================
// STEP 13: ADMIN DASHBOARD (DormHub)
// Admin role is verified in the database for EVERY admin request.
// Only role management is writable. No deletion of user/dorm data.
// =========================================================

type AdminOverview struct {
	Name            string
	Users           int
	Landlords       int
	Dorms           int
	Rooms           int
	AvailableRooms  int
	PendingRequests int
	Reviews         int
	RecentDorms     []AdminDorm
	RecentRequests  []AdminRequest
}

type AdminUser struct {
	ID        int
	Name      string
	Phone     string
	Role      string
	CreatedAt string
	DormCount int
}

type AdminUsersPage struct {
	Name      string
	CSRFToken string
	Users     []AdminUser
	Result    string
}

type AdminDorm struct {
	ID             int
	Name           string
	Address        string
	OwnerName      string
	TotalRooms     int
	AvailableRooms int
}

type AdminDormsPage struct {
	Name  string
	Dorms []AdminDorm
}

type AdminRequest struct {
	ID         int
	TenantName string
	DormName   string
	RoomNumber string
	Status     string
	MoveInDate string
	CreatedAt  string
}

type AdminRequestsPage struct {
	Name     string
	Requests []AdminRequest
}

func adminRequire(w http.ResponseWriter, r *http.Request) (int, string, *sessions.Session, bool) {
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	var id int
	switch v := session.Values["user_id"].(type) {
	case int:
		id = v
	case int64:
		id = int(v)
	}
	if id <= 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	var name, role string
	err = db.QueryRowContext(r.Context(), "SELECT name, role FROM users WHERE id = ?", id).Scan(&name, &role)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return 0, "", nil, false
	}
	if err != nil {
		log.Println("Admin authorization error:", err)
		http.Error(w, "เน€เธเธดเธ”เธเนเธญเธเธดเธ”เธเธฅเธฒเธ”เนเธเธเธฒเธฃเธ•เธฃเธงเธเธชเธญเธเธชเธดเธ—เธเธดเน", http.StatusInternalServerError)
		return 0, "", nil, false
	}
	if role != "admin" {
		http.Error(w, "403 Forbidden: เธชเธณเธซเธฃเธฑเธเธเธนเนเธ”เธนเนเธฅเธฃเธฐเธเธเน€เธ—เนเธฒเธเธฑเนเธ", http.StatusForbidden)
		return 0, "", nil, false
	}
	return id, name, session, true
}

func adminRender(w http.ResponseWriter, filename string, data any) {
	t, err := template.ParseFiles("templates/" + filename)
	if err != nil {
		log.Println("Admin template parse error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธซเธเนเธฒ Admin เนเธกเนเธชเธณเน€เธฃเนเธ", http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, data); err != nil {
		log.Println("Admin template render error:", err)
	}
}

func adminGET(w http.ResponseWriter, r *http.Request, exactPath string) bool {
	if r.URL.Path != exactPath {
		http.NotFound(w, r)
		return false
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func adminReadDorms(r *http.Request, limit int) ([]AdminDorm, error) {
	rows, err := db.QueryContext(r.Context(), `
        SELECT d.id,d.name,d.address,u.name,
          (SELECT COUNT(*) FROM rooms rm WHERE rm.dormitory_id=d.id),
          (SELECT COUNT(*) FROM rooms rm WHERE rm.dormitory_id=d.id AND rm.status='available')
        FROM dormitories d JOIN users u ON u.id=d.owner_id
        ORDER BY d.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AdminDorm{}
	for rows.Next() {
		var x AdminDorm
		if err = rows.Scan(&x.ID, &x.Name, &x.Address, &x.OwnerName, &x.TotalRooms, &x.AvailableRooms); err != nil {
			return nil, err
		}
		result = append(result, x)
	}
	return result, rows.Err()
}

func adminReadRequests(r *http.Request, limit int) ([]AdminRequest, error) {
	rows, err := db.QueryContext(r.Context(), `
        SELECT rr.id,u.name,d.name,rm.room_number,rr.status,
               DATE_FORMAT(rr.move_in_date,'%d/%m/%Y'),
               DATE_FORMAT(rr.created_at,'%d/%m/%Y %H:%i')
        FROM rental_requests rr
        JOIN users u ON u.id=rr.user_id
        JOIN rooms rm ON rm.id=rr.room_id
        JOIN dormitories d ON d.id=rm.dormitory_id
        ORDER BY rr.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AdminRequest{}
	for rows.Next() {
		var x AdminRequest
		if err = rows.Scan(&x.ID, &x.TenantName, &x.DormName, &x.RoomNumber, &x.Status, &x.MoveInDate, &x.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, x)
	}
	return result, rows.Err()
}

// GET /admin: overview only; no changes to dorm, request or review data.
func adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	if !adminGET(w, r, "/admin") {
		return
	}
	_, name, _, ok := adminRequire(w, r)
	if !ok {
		return
	}
	data := AdminOverview{Name: name}
	err := db.QueryRowContext(r.Context(), `
        SELECT
          (SELECT COUNT(*) FROM users),
          (SELECT COUNT(*) FROM users WHERE role='landlord'),
          (SELECT COUNT(*) FROM dormitories),
          (SELECT COUNT(*) FROM rooms),
          (SELECT COUNT(*) FROM rooms WHERE status='available'),
          (SELECT COUNT(*) FROM rental_requests WHERE status='pending'),
          (SELECT COUNT(*) FROM reviews)
    `).Scan(&data.Users, &data.Landlords, &data.Dorms, &data.Rooms,
		&data.AvailableRooms, &data.PendingRequests, &data.Reviews)
	if err != nil {
		log.Println("Admin dashboard stats error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธชเธ–เธดเธ•เธดเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	data.RecentDorms, err = adminReadDorms(r, 5)
	if err != nil {
		log.Println("Admin dorms error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเนเธญเธกเธนเธฅเธซเธญเธเธฑเธเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	data.RecentRequests, err = adminReadRequests(r, 5)
	if err != nil {
		log.Println("Admin requests error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธณเธเธญเน€เธเนเธฒเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	adminRender(w, "admin.html", data)
}

// A persistent session-bound random CSRF token, shared only with admin role pages.
func adminCSRFToken(w http.ResponseWriter, r *http.Request, s *sessions.Session) (string, error) {
	if existing, ok := s.Values["admin_csrf"].(string); ok && existing != "" {
		return existing, nil
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	s.Values["admin_csrf"] = token
	if err := s.Save(r, w); err != nil {
		return "", err
	}
	return token, nil
}

// GET /admin/users: list; POST: user <-> landlord only.
// Admin accounts cannot be altered from the web UI.
func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin/users" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	actorID, name, session, ok := adminRequire(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "เธเนเธญเธกเธนเธฅเธเธญเธฃเนเธกเธเธดเธ”เธเธฅเธฒเธ”", 400)
			return
		}
		expected, ok := session.Values["admin_csrf"].(string)
		submitted := r.PostFormValue("csrf_token")
		if !ok || expected == "" || submitted == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) != 1 {
			http.Error(w, "403 Forbidden: CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ เธเธฃเธธเธ“เธฒเน€เธเธดเธ”เธซเธเนเธฒเนเธซเธกเน", 403)
			return
		}
		id, err := strconv.Atoi(r.PostFormValue("user_id"))
		role := r.PostFormValue("role")
		if err != nil || id <= 0 || (role != "user" && role != "landlord") {
			http.Error(w, "เธเนเธญเธกเธนเธฅเธเธนเนเนเธเนเธซเธฃเธทเธญเธชเธดเธ—เธเธดเนเนเธกเนเธ–เธนเธเธ•เนเธญเธ", 400)
			return
		}
		if id == actorID {
			http.Error(w, "เนเธกเนเธญเธเธธเธเธฒเธ•เนเธซเนเนเธเนเธชเธดเธ—เธเธดเนเธ•เธเน€เธญเธ", 403)
			return
		}
		// Conditional update: never changes admin accounts; never downgrades an owner
		// while they have dorms, so ownership is never orphaned by a role change.
		result, err := db.ExecContext(r.Context(), `
            UPDATE users u SET role=?
            WHERE u.id=? AND u.role IN ('user','landlord')
              AND (?='landlord' OR NOT EXISTS
                   (SELECT 1 FROM dormitories d WHERE d.owner_id=u.id))`, role, id, role)
		if err != nil {
			log.Println("Admin role update error:", err)
			http.Error(w, "เน€เธเธฅเธตเนเธขเธเธชเธดเธ—เธเธดเนเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		affected, err := result.RowsAffected()
		if err != nil {
			log.Println("Admin rows affected error:", err)
			http.Error(w, "เธ•เธฃเธงเธเธเธฅเธเธฒเธฃเนเธเนเนเธเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		if affected == 0 {
			// Nothing changed, or protected user/dorm owner. Explain on next GET.
			http.Redirect(w, r, "/admin/users?result=unchanged", http.StatusSeeOther)
			return
		}
		log.Printf("Admin ID=%d changed user ID=%d role to %s", actorID, id, role)
		http.Redirect(w, r, "/admin/users?result=updated", http.StatusSeeOther)
		return
	}
	token, err := adminCSRFToken(w, r, session)
	if err != nil {
		log.Println("Admin csrf error:", err)
		http.Error(w, "เธชเธฃเนเธฒเธ CSRF Token เนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	rows, err := db.QueryContext(r.Context(), `
        SELECT u.id,u.name,u.phone,u.role,DATE_FORMAT(u.created_at,'%d/%m/%Y'),
          (SELECT COUNT(*) FROM dormitories d WHERE d.owner_id=u.id)
        FROM users u ORDER BY u.id DESC LIMIT 200`)
	if err != nil {
		log.Println("Admin users query error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธนเนเนเธเนเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	data := AdminUsersPage{Name: name, CSRFToken: token, Users: []AdminUser{}, Result: r.URL.Query().Get("result")}
	for rows.Next() {
		var x AdminUser
		if err = rows.Scan(&x.ID, &x.Name, &x.Phone, &x.Role, &x.CreatedAt, &x.DormCount); err != nil {
			break
		}
		data.Users = append(data.Users, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		log.Println("Admin users scan error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธนเนเนเธเนเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	adminRender(w, "admin_users.html", data)
}

func adminDormsHandler(w http.ResponseWriter, r *http.Request) {
	if !adminGET(w, r, "/admin/dorms") {
		return
	}
	_, name, _, ok := adminRequire(w, r)
	if !ok {
		return
	}
	dorms, err := adminReadDorms(r, 200)
	if err != nil {
		log.Println("Admin dorms list error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธซเธญเธเธฑเธเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	adminRender(w, "admin_dorms.html", AdminDormsPage{Name: name, Dorms: dorms})
}

func adminRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if !adminGET(w, r, "/admin/requests") {
		return
	}
	_, name, _, ok := adminRequire(w, r)
	if !ok {
		return
	}
	requests, err := adminReadRequests(r, 200)
	if err != nil {
		log.Println("Admin requests list error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธณเธเธญเน€เธเนเธฒเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	adminRender(w, "admin_requests.html", AdminRequestsPage{Name: name, Requests: requests})
}

// ============================================================================
// STEP 15 - DEMO-ONLY LANDLORD DOCUMENT VERIFICATION
// Documents stored outside /static; never use real identification documents.
// ============================================================================

var demoDocsDir = uploadedDocumentDir()

// Strictly validate file contents rather than trusting the filename/content-type.
func saveDemoLandlordDocument(upload multipart.File, header *multipart.FileHeader) (string, error) {
	const max int64 = 3 << 20
	if header == nil || header.Size < 1 || header.Size > max {
		return "", errors.New("เน€เธญเธเธชเธฒเธฃเธ—เธ”เธชเธญเธเธ•เนเธญเธเธกเธตเธเธเธฒเธ”เนเธกเนเน€เธเธดเธ 3 MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".pdf" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return "", errors.New("เธฃเธญเธเธฃเธฑเธเน€เธญเธเธชเธฒเธฃเธ•เธฑเธงเธญเธขเนเธฒเธ .txt, .pdf, .png, .jpg เน€เธ—เนเธฒเธเธฑเนเธ")
	}
	contents, err := io.ReadAll(io.LimitReader(upload, max+1))
	if err != nil || len(contents) == 0 || len(contents) > int(max) {
		return "", errors.New("เธญเนเธฒเธเนเธเธฅเนเน€เธญเธเธชเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธเธซเธฃเธทเธญเนเธเธฅเนเนเธซเธเนเน€เธเธดเธ 3 MB")
	}
	switch ext {
	case ".txt":
		if !utf8.Valid(contents) || bytes.IndexByte(contents, 0) >= 0 {
			return "", errors.New("เนเธเธฅเน .txt เธ•เนเธญเธเน€เธเนเธเธเนเธญเธเธงเธฒเธก UTF-8")
		}
	case ".pdf":
		if !bytes.HasPrefix(contents, []byte("%PDF-")) {
			return "", errors.New("เน€เธญเธเธชเธฒเธฃ .pdf เนเธกเนเธ–เธนเธเธ•เนเธญเธ")
		}
	case ".png":
		if _, _, err := image.DecodeConfig(bytes.NewReader(contents)); err != nil {
			return "", errors.New("เนเธเธฅเนเธ เธฒเธ PNG เนเธกเนเธ–เธนเธเธ•เนเธญเธ")
		}
		if !bytes.HasPrefix(contents, []byte("\x89PNG\r\n\x1a\n")) {
			return "", errors.New("เธฃเธนเธเนเธเธเนเธเธฅเน PNG เนเธกเนเธ–เธนเธเธ•เนเธญเธ")
		}
	case ".jpg", ".jpeg":
		if len(contents) < 3 || !bytes.Equal(contents[:3], []byte{0xff, 0xd8, 0xff}) {
			return "", errors.New("เนเธเธฅเน JPEG เนเธกเนเธ–เธนเธเธ•เนเธญเธ")
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(contents)); err != nil {
			return "", errors.New("เนเธเธฅเน JPEG เนเธกเนเธ–เธนเธเธ•เนเธญเธ")
		}
	}
	if err := os.MkdirAll(demoDocsDir, 0700); err != nil {
		return "", errors.New("เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธชเธฃเนเธฒเธเนเธเธฅเน€เธ”เธญเธฃเนเน€เธญเธเธชเธฒเธฃเนเธ”เน")
	}
	f, err := os.CreateTemp(demoDocsDir, "proof-*")
	if err != nil {
		return "", errors.New("เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธเธฑเธเธ—เธถเธเน€เธญเธเธชเธฒเธฃเนเธ”เน")
	}
	name := f.Name()
	defer func() { f.Close() }()
	if err := f.Chmod(0600); err != nil {
		os.Remove(name)
		return "", errors.New("เธเธณเธซเธเธ”เธชเธดเธ—เธเธดเนเนเธเธฅเนเนเธกเนเธชเธณเน€เธฃเนเธ")
	}
	if _, err := f.Write(contents); err != nil {
		os.Remove(name)
		return "", errors.New("เน€เธเธตเธขเธเนเธเธฅเนเน€เธญเธเธชเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธ")
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", errors.New("เธเธดเธ”เนเธเธฅเนเน€เธญเธเธชเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธ")
	}
	target := name + ext
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return "", errors.New("เธ•เธฑเนเธเธเธทเนเธญเนเธเธฅเนเน€เธญเธเธชเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธ")
	}
	return target, nil
}

type demoLandlordStatusPage struct {
	Name, DormName, Status, ReviewNote string
}

func landlordApplicationStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/landlord/application-status" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", 405)
		return
	}
	session, err := store.Get(r, "dormhub-session")
	if err != nil {
		http.Redirect(w, r, "/login", 303)
		return
	}
	id, ok := session.Values["user_id"].(int)
	if !ok || id <= 0 {
		http.Redirect(w, r, "/login", 303)
		return
	}
	var role, name string
	var page demoLandlordStatusPage
	err = db.QueryRowContext(r.Context(), `SELECT u.name,u.role,a.dorm_name,a.status,COALESCE(a.review_note,'')
        FROM users u JOIN landlord_applications a ON a.user_id=u.id WHERE u.id=?`, id).
		Scan(&name, &role, &page.DormName, &page.Status, &page.ReviewNote)
	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/dashboard", 303)
		return
	}
	if err != nil {
		log.Println("Landlord status error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธชเธ–เธฒเธเธฐเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	page.Name = name
	if role == "landlord" && page.Status == "approved" {
		// The page still explains that approval was completed; login again to refresh session.
	}
	adminRender(w, "landlord_application_status.html", page)
}

type demoLandlordApplication struct {
	ID                                                                                     int
	UserName, Phone, DormName, Status, OriginalFilename, CreatedAt, ReviewedAt, ReviewNote string
}
type demoLandlordApplicationsPage struct {
	Name, CSRFToken, Result string
	Applications            []demoLandlordApplication
}

func adminLandlordApplicationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin/landlord-applications" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", 405)
		return
	}
	actorID, name, session, ok := adminRequire(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "เธเนเธญเธกเธนเธฅเนเธกเนเธ–เธนเธเธ•เนเธญเธ", 400)
			return
		}
		token, ok := session.Values["admin_csrf"].(string)
		posted := r.PostFormValue("csrf_token")
		if !ok || token == "" || posted == "" || subtle.ConstantTimeCompare([]byte(token), []byte(posted)) != 1 {
			http.Error(w, "CSRF Token เนเธกเนเธ–เธนเธเธ•เนเธญเธ", 403)
			return
		}
		appID, err := strconv.Atoi(r.PostFormValue("application_id"))
		decision := r.PostFormValue("decision")
		note := strings.TrimSpace(r.PostFormValue("review_note"))
		if err != nil || appID <= 0 || (decision != "approved" && decision != "rejected") || len([]rune(note)) > 255 {
			http.Error(w, "เธเนเธญเธกเธนเธฅเธเธฒเธฃเธญเธเธธเธกเธฑเธ•เธดเนเธกเนเธ–เธนเธเธ•เนเธญเธ", 400)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "เน€เธเธดเธ”เธฃเธฒเธขเธเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		defer tx.Rollback()
		var targetID int
		var status string
		err = tx.QueryRowContext(r.Context(), "SELECT user_id,status FROM landlord_applications WHERE id=? FOR UPDATE", appID).Scan(&targetID, &status)
		if err == sql.ErrNoRows {
			http.Error(w, "เนเธกเนเธเธเธเธณเธเธญ", 404)
			return
		}
		if err != nil {
			http.Error(w, "เธ•เธฃเธงเธเธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		if status != "pending" {
			http.Redirect(w, r, "/admin/landlord-applications?result=unchanged", 303)
			return
		}
		if targetID == actorID {
			http.Error(w, "เนเธกเนเธชเธฒเธกเธฒเธฃเธ–เธญเธเธธเธกเธฑเธ•เธดเธเธณเธเธญเธเธญเธเธ•เธเน€เธญเธ", 403)
			return
		}
		if decision == "approved" {
			result, err := tx.ExecContext(r.Context(), "UPDATE users SET role='landlord' WHERE id=? AND role='user'", targetID)
			if err != nil {
				log.Println("Landlord approval user update error:", err)
				http.Error(w, "เน€เธเธฅเธตเนเธขเธเธชเธดเธ—เธเธดเนเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
				return
			}
			affected, err := result.RowsAffected()
			if err != nil || affected != 1 {
				http.Error(w, "เธเธฑเธเธเธตเนเธกเนเธญเธขเธนเนเนเธเธชเธ–เธฒเธเธฐ user เธเธถเธเธญเธเธธเธกเธฑเธ•เธดเนเธกเนเนเธ”เน", 409)
				return
			}
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE landlord_applications
           SET status=?,review_note=?,reviewed_by=?,reviewed_at=NOW()
           WHERE id=? AND status='pending'`, decision, note, actorID, appID)
		if err != nil {
			log.Println("Admin landlord application update:", err)
			http.Error(w, "เธเธฑเธเธ—เธถเธเธเธฅเธเธฒเธฃเธ•เธฃเธงเธเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "เธเธฑเธเธ—เธถเธเธเธฅเธ•เธฃเธงเธเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
			return
		}
		http.Redirect(w, r, "/admin/landlord-applications?result="+decision, 303)
		return
	}
	token, err := adminCSRFToken(w, r, session)
	if err != nil {
		http.Error(w, "เธชเธฃเนเธฒเธ CSRF Token เนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT a.id,u.name,u.phone,a.dorm_name,a.status,
        a.original_filename,DATE_FORMAT(a.created_at,'%d/%m/%Y %H:%i'),
        COALESCE(DATE_FORMAT(a.reviewed_at,'%d/%m/%Y %H:%i'),''),COALESCE(a.review_note,'')
        FROM landlord_applications a JOIN users u ON u.id=a.user_id
        ORDER BY FIELD(a.status,'pending','rejected','approved'),a.id DESC LIMIT 200`)
	if err != nil {
		log.Println("Applications list error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ เธ•เธฃเธงเธเธชเธญเธเธเธฒเธฃ Import SQL", 500)
		return
	}
	page := demoLandlordApplicationsPage{Name: name, CSRFToken: token, Result: r.URL.Query().Get("result"), Applications: []demoLandlordApplication{}}
	for rows.Next() {
		var x demoLandlordApplication
		if err = rows.Scan(&x.ID, &x.UserName, &x.Phone, &x.DormName, &x.Status, &x.OriginalFilename, &x.CreatedAt, &x.ReviewedAt, &x.ReviewNote); err != nil {
			break
		}
		page.Applications = append(page.Applications, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		log.Println("Applications scan error:", err)
		http.Error(w, "เนเธซเธฅเธ”เธเธณเธเธญเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	adminRender(w, "admin_landlord_applications.html", page)
}

func adminLandlordDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin/landlord-document" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", 405)
		return
	}
	_, _, _, ok := adminRequire(w, r)
	if !ok {
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "เธฃเธซเธฑเธชเน€เธญเธเธชเธฒเธฃเนเธกเนเธ–เธนเธเธ•เนเธญเธ", 400)
		return
	}
	var docPath string
	err = db.QueryRowContext(r.Context(), "SELECT document_path FROM landlord_applications WHERE id=?", id).Scan(&docPath)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "เนเธซเธฅเธ”เน€เธญเธเธชเธฒเธฃเนเธกเนเธชเธณเน€เธฃเนเธ", 500)
		return
	}
	clean := filepath.Clean(filepath.FromSlash(docPath))
	if filepath.Dir(clean) != filepath.Clean(demoDocsDir) || !strings.HasPrefix(filepath.Base(clean), "proof-") {
		http.Error(w, "เธ•เธณเนเธซเธเนเธเนเธเธฅเนเนเธกเนเธ–เธนเธเธ•เนเธญเธ", 403)
		return
	}
	f, err := os.Open(clean)
	if err != nil {
		http.Error(w, "เนเธกเนเธเธเนเธเธฅเนเน€เธญเธเธชเธฒเธฃ", 404)
		return
	}
	defer f.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/octet-stream")
	// Never render untrusted documents inline in the app origin.
	ext := strings.ToLower(filepath.Ext(clean))
	w.Header().Set("Content-Disposition", "attachment; filename=landlord-demo-document"+ext)
	if _, err := io.Copy(w, f); err != nil {
		log.Println("Landlord document stream error:", err)
	}
}
