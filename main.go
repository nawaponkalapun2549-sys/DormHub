package main

import (
	"bytes"
	"crypto/rand"

	"crypto/subtle"

	"database/sql"
	"fmt"

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

		log.Fatal("เปิด Database ไม่สำเร็จ:", err)

	}

	if err = db.Ping(); err != nil {

		log.Fatal("เชื่อมต่อ Database ไม่สำเร็จ:", err)

	}

	log.Println("=================================")

	log.Println("✅ DormHub Database Connected")

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

	log.Println("🚀 DormHub server starting")

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

			"ไม่สามารถโหลดหน้าเว็บได้",

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
		showRegisterError(w, "ข้อมูลหรือเอกสารมีขนาดใหญ่เกินกำหนด (สูงสุด 3 MB)")
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
		showRegisterError(w, "กรุณาเลือกประเภทบัญชี user หรือ landlord")
		return
	}
	if name == "" || len([]rune(name)) > 100 || len(phone) != 10 || password == "" || confirm == "" {
		showRegisterError(w, "กรุณาตรวจสอบชื่อ เบอร์โทร 10 หลัก และรหัสผ่านให้ครบ")
		return
	}
	for _, c := range phone {
		if c < '0' || c > '9' {
			showRegisterError(w, "เบอร์โทรต้องเป็นตัวเลข 10 หลัก")
			return
		}
	}
	if len(password) < 6 || password != confirm {
		showRegisterError(w, "รหัสผ่านต้องมีอย่างน้อย 6 ตัวอักษรและยืนยันให้ตรงกัน")
		return
	}

	var document multipart.File
	var header *multipart.FileHeader
	dormName := ""
	if accountType == "landlord" {
		dormName = strings.TrimSpace(r.FormValue("dorm_name"))
		if dormName == "" || len([]rune(dormName)) > 150 {
			showRegisterError(w, "กรุณากรอกชื่อหอพักตัวอย่างไม่เกิน 150 ตัวอักษร")
			return
		}
		var err error
		document, header, err = r.FormFile("proof_document")
		if err != nil {
			showRegisterError(w, "กรุณาแนบไฟล์เอกสารทดสอบ")
			return
		}
		defer document.Close()
		if header.Size == 0 || header.Size > 3<<20 {
			showRegisterError(w, "เอกสารต้องมีขนาด 1 ไบต์ ถึง 3 MB")
			return
		}
	}
	var existing int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE phone=?", phone).Scan(&existing); err != nil {
		log.Println("Register user check error:", err)
		showRegisterError(w, "ไม่สามารถตรวจสอบเบอร์โทรได้")
		return
	}
	if existing != 0 {
		showRegisterError(w, "เบอร์โทรนี้ถูกใช้งานแล้ว")
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		showRegisterError(w, "ไม่สามารถสร้างรหัสผ่านได้")
		return
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Register tx error:", err)
		showRegisterError(w, "ฐานข้อมูลไม่พร้อมใช้งาน")
		return
	}
	defer tx.Rollback()
	// Everybody starts as user. Only a reviewing admin can grant landlord role.
	inserted, err := tx.ExecContext(r.Context(), "INSERT INTO users (name,phone,password_hash,role) VALUES (?,?,?,'user')", name, phone, string(hashed))
	if err != nil {
		log.Println("Register insert error:", err)
		showRegisterError(w, "ไม่สามารถสร้างบัญชีได้ (เบอร์อาจซ้ำ)")
		return
	}
	userID, err := inserted.LastInsertId()
	if err != nil {
		showRegisterError(w, "ไม่สามารถสร้างบัญชีได้")
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
			showRegisterError(w, "บันทึกคำขอไม่สำเร็จ กรุณาตรวจสอบว่า Import SQL ขั้นตอน 15 แล้ว")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Println("Register commit error:", err)
		showRegisterError(w, "บันทึกการสมัครไม่สำเร็จ")
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
		http.Error(w, "โหลดหน้าสมัครไม่ได้", 500)
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

				"ไม่สามารถโหลดหน้า Login ได้",

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
			data.Success = "สมัครสมาชิกสำเร็จ! กรุณาเข้าสู่ระบบ"
			if r.URL.Query().Get("landlord") == "pending" {
				data.Success = "สมัครบัญชีเจ้าของหอสำเร็จ! เอกสารตัวอย่างอยู่ระหว่างรอ Admin ตรวจสอบ กรุณา Login เพื่อดูสถานะ"
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

			"ข้อมูลไม่ถูกต้อง",

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

			"กรุณากรอกเบอร์โทรและรหัสผ่าน",
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

			"เบอร์โทรหรือรหัสผ่านไม่ถูกต้อง",
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

			"เบอร์โทรหรือรหัสผ่านไม่ถูกต้อง",
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

			"ไม่สามารถสร้าง Session ได้",

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

			"ไม่สามารถบันทึก Session ได้",

			http.StatusInternalServerError,
		)

		return

	}

	// =========================

	// LOG

	// =========================

	log.Printf(

		"Login สำเร็จ: ID=%d Name=%s Phone=%s Role=%s",

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

	log.Println("Logout สำเร็จ")

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

		name = "ผู้ใช้งาน"

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

			"ไม่สามารถโหลด Dashboard ได้",

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

	case "มหาสารคาม":

		query += " AND d.address LIKE ?"

		args = append(args, "%มหาสารคาม%")

	case "มมส":

		query += " AND (d.address LIKE ? OR d.address LIKE ?)"

		args = append(args, "%มมส%", "%มหาวิทยาลัยมหาสารคาม%")

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

		args = append(args, "%air%", "%แอร์%")

	case "fan":

		query += " AND (LOWER(r.room_type) LIKE ? OR r.room_type LIKE ?)"

		args = append(args, "%fan%", "%พัดลม%")

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

		http.Error(w, "เกิดข้อผิดพลาดในการค้นหาหอพัก", http.StatusInternalServerError)

		return

	}

	defer rows.Close()

	for rows.Next() {

		var dorm DormSearchResult

		if err := rows.Scan(&dorm.ID, &dorm.Name, &dorm.Address,

			&dorm.Description, &dorm.Image, &dorm.MinPrice,

			&dorm.AvailableRooms); err != nil {

			log.Println("Search Scan Error:", err)

			http.Error(w, "ไม่สามารถอ่านข้อมูลหอพักได้", http.StatusInternalServerError)

			return

		}

		data.Dorms = append(data.Dorms, dorm)

	}

	if err := rows.Err(); err != nil {

		log.Println("Search Rows Error:", err)

		http.Error(w, "เกิดข้อผิดพลาดในการอ่านผลค้นหา", http.StatusInternalServerError)

		return

	}

	data.Count = len(data.Dorms)

	tmpl, err := template.ParseFiles("templates/search.html")

	if err != nil {

		log.Println("Search Template Error:", err)

		http.Error(w, "ไม่สามารถโหลดหน้าค้นหาได้", http.StatusInternalServerError)

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

	// ตรวจสอบ URL

	if r.URL.Path != "/landlord" {

		http.NotFound(w, r)

		return

	}

	// อนุญาตเฉพาะ GET

	if r.Method != http.MethodGet {

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

		return

	}

	// =========================

	// ตรวจสอบ SESSION

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

	// ตรวจสิทธิ์จาก DATABASE

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

		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

		return

	}

	// เฉพาะเจ้าของหอพักเท่านั้น

	if role != "landlord" {

		http.Error(

			w,

			"403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น",

			http.StatusForbidden,
		)

		return

	}

	// =========================

	// โครงสร้างข้อมูลหอพัก

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

	// ดึงเฉพาะหอพักของเจ้าของ

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

		http.Error(w, "ไม่สามารถโหลดข้อมูลหอพักได้", http.StatusInternalServerError)

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

			http.Error(w, "ไม่สามารถอ่านข้อมูลหอพักได้", http.StatusInternalServerError)

			return

		}

		data.Dorms = append(data.Dorms, dorm)

		data.RoomCount += dorm.RoomCount

	}

	if err := rows.Err(); err != nil {

		rows.Close()

		log.Println("Landlord Dorm Rows Error:", err)

		http.Error(w, "เกิดข้อผิดพลาดในการอ่านข้อมูลหอพัก", http.StatusInternalServerError)

		return

	}

	rows.Close()

	// จำนวนหอพักของตัวเอง

	data.DormCount = len(data.Dorms)

	// =========================

	// นับคำขอเช่าที่รอตรวจสอบ

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

		http.Error(w, "ไม่สามารถโหลดข้อมูลคำขอเช่าได้", http.StatusInternalServerError)

		return

	}

	// =========================

	// โหลด LANDLORD.HTML

	// =========================

	tmpl, err := template.ParseFiles(

		"templates/landlord.html",
	)

	if err != nil {

		log.Println("Landlord Template Error:", err)

		http.Error(w, "ไม่สามารถโหลดหน้า Landlord ได้", http.StatusInternalServerError)

		return

	}

	// =========================

	// แสดงหน้า Dashboard

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

		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(w, "403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น", http.StatusForbidden)

		return

	}

	// GET: generate a random CSRF token bound to this user's session.

	if r.Method == http.MethodGet {

		tokenBytes := make([]byte, 32)

		if _, err := rand.Read(tokenBytes); err != nil {

			log.Println("Landlord Add CSRF Generation Error:", err)

			http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

			return

		}

		token := hex.EncodeToString(tokenBytes)

		session.Values["landlord_add_csrf"] = token

		if err := session.Save(r, w); err != nil {

			log.Println("Landlord Add Session Save Error:", err)

			http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)

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

		http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้องหรือมีขนาดใหญ่เกินไป", http.StatusBadRequest)

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

		http.Error(w, "403 Forbidden: CSRF Token ไม่ถูกต้อง กรุณาเปิดฟอร์มใหม่", http.StatusForbidden)

		return

	}

	dormName := strings.TrimSpace(r.PostFormValue("dorm_name"))

	address := strings.TrimSpace(r.PostFormValue("address"))

	description := strings.TrimSpace(r.PostFormValue("description"))

	if dormName == "" || address == "" {

		renderLandlordAdd(w, "กรุณากรอกชื่อหอพักและที่อยู่", savedToken)

		return

	}

	if utf8.RuneCountInString(dormName) > 150 {

		renderLandlordAdd(w, "ชื่อหอพักต้องไม่เกิน 150 ตัวอักษร", savedToken)

		return

	}

	if utf8.RuneCountInString(address) > 2000 {

		renderLandlordAdd(w, "ที่อยู่ยาวเกินกำหนด", savedToken)

		return

	}

	if utf8.RuneCountInString(description) > 5000 {

		renderLandlordAdd(w, "รายละเอียดหอพักยาวเกินกำหนด", savedToken)

		return

	}

	// Optional local photo: only upload after CSRF and the text fields pass validation.
	imageURL := ""
	imageFile, imageHeader, fileErr := r.FormFile("image_file")
	if fileErr != nil && !errors.Is(fileErr, http.ErrMissingFile) {
		renderLandlordAdd(w, "ไม่สามารถอ่านไฟล์รูปภาพได้ กรุณาเลือกไฟล์อีกครั้ง", savedToken)
		return
	}
	if fileErr == nil {
		defer imageFile.Close()
		// Protect against oversized uploads before touching the file system.
		if imageHeader.Size > 5<<20 {
			renderLandlordAdd(w, "รูปภาพต้องมีขนาดไม่เกิน 5 MB", savedToken)
			return
		}
		imageURL, err = saveDormImage(imageFile)
		if err != nil {
			log.Println("Landlord Add Image Upload Error:", err)
			renderLandlordAdd(w, "อัปโหลดรูปไม่สำเร็จ ใช้ไฟล์ JPG, PNG, GIF หรือ WebP ขนาดไม่เกิน 5 MB", savedToken)
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

		renderLandlordAdd(w, "ไม่สามารถบันทึกหอพักได้ กรุณาลองอีกครั้ง", savedToken)

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

		http.Error(w, "ไม่สามารถโหลดหน้าฟอร์มได้", http.StatusInternalServerError)

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

		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(w, "403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น", http.StatusForbidden)

		return

	}

	var dormID int

	if r.Method == http.MethodGet {

		dormID, err = strconv.Atoi(r.URL.Query().Get("dorm_id"))

	} else {

		r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

		if err := r.ParseForm(); err != nil {

			http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้องหรือมีขนาดใหญ่เกินไป", http.StatusBadRequest)

			return

		}

		dormID, err = strconv.Atoi(r.PostFormValue("dorm_id"))

	}

	if err != nil || dormID <= 0 {

		http.Error(w, "กรุณาระบุรหัสหอพักให้ถูกต้อง", http.StatusBadRequest)

		return

	}

	// Never trust the hidden dorm_id alone. Query using authenticated owner_id.

	var dormName string

	err = db.QueryRowContext(r.Context(),

		"SELECT name FROM dormitories WHERE id = ? AND owner_id = ?", dormID, userID,
	).Scan(&dormName)

	if err == sql.ErrNoRows {

		http.Error(w, "ไม่พบหอพัก หรือคุณไม่มีสิทธิ์จัดการหอพักนี้", http.StatusForbidden)

		return

	}

	if err != nil {

		log.Println("Room Add Dorm Query Error:", err)

		http.Error(w, "ไม่สามารถอ่านข้อมูลหอพักได้", http.StatusInternalServerError)

		return

	}

	if r.Method == http.MethodGet {

		tokenBytes := make([]byte, 32)

		if _, err := rand.Read(tokenBytes); err != nil {

			log.Println("Room Add CSRF Error:", err)

			http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

			return

		}

		csrfToken := hex.EncodeToString(tokenBytes)

		session.Values["landlord_room_add_csrf"] = csrfToken

		if err := session.Save(r, w); err != nil {

			log.Println("Room Add Session Error:", err)

			http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)

			return

		}

		renderLandlordRoomAdd(w, dormID, dormName, csrfToken, "")

		return

	}

	savedToken, ok := session.Values["landlord_room_add_csrf"].(string)

	submittedToken := r.PostFormValue("csrf_token")

	if !ok || savedToken == "" || subtle.ConstantTimeCompare([]byte(savedToken), []byte(submittedToken)) != 1 {

		http.Error(w, "403 Forbidden: CSRF Token ไม่ถูกต้อง กรุณาเปิดฟอร์มใหม่", http.StatusForbidden)

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

		showError("กรุณากรอกเลขห้องไม่เกิน 20 ตัวอักษร")

		return

	}

	if roomType != "air" && roomType != "fan" {

		showError("กรุณาเลือกประเภทห้องให้ถูกต้อง")

		return

	}

	if status != "available" && status != "occupied" && status != "maintenance" {

		showError("กรุณาเลือกสถานะห้องให้ถูกต้อง")

		return

	}

	price, priceErr := strconv.ParseFloat(priceText, 64)

	if priceErr != nil || math.IsNaN(price) || math.IsInf(price, 0) ||

		price <= 0 || price > 99999999.99 || math.Abs(price*100-math.Round(price*100)) > 0.00001 {

		showError("กรุณากรอกราคาให้ถูกต้อง (มากกว่า 0 และทศนิยมไม่เกิน 2 ตำแหน่ง)")

		return

	}

	if utf8.RuneCountInString(description) > 5000 {

		showError("รายละเอียดห้องยาวเกินกำหนด")

		return

	}

	var existingCount int

	err = db.QueryRowContext(r.Context(),

		"SELECT COUNT(*) FROM rooms WHERE dormitory_id = ? AND room_number = ?",

		dormID, roomNumber,
	).Scan(&existingCount)

	if err != nil {

		log.Println("Room Add Duplicate Query Error:", err)

		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

		return

	}

	if existingCount > 0 {

		showError("เลขห้องนี้มีอยู่ในหอพักแล้ว")

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

		showError("บันทึกห้องพักไม่สำเร็จ หรือเลขห้องซ้ำ กรุณาลองอีกครั้ง")

		return

	}

	affected, err := result.RowsAffected()

	if err != nil || affected != 1 {

		http.Error(w, "ไม่มีสิทธิ์เพิ่มห้องพักในหอนี้", http.StatusForbidden)

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

		http.Error(w, "ไม่สามารถโหลดหน้าเพิ่มห้องพักได้", http.StatusInternalServerError)

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

	// ตรวจสอบ URL

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

		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)

		return

	}

	if role != "landlord" {

		http.Error(

			w,

			"403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น",

			http.StatusForbidden,
		)

		return

	}

	// =========================

	// GET DORM ID

	// =========================

	dormID, err := strconv.Atoi(r.URL.Query().Get("dorm_id"))

	if err != nil || dormID <= 0 {

		http.Error(w, "รหัสหอพักไม่ถูกต้อง", http.StatusBadRequest)

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

		http.Error(w, "ไม่สามารถตรวจสอบหอพักได้", http.StatusInternalServerError)

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

		http.Error(w, "ไม่สามารถโหลดห้องพักได้", http.StatusInternalServerError)

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

			http.Error(w, "ไม่สามารถอ่านข้อมูลห้องพักได้", http.StatusInternalServerError)

			return

		}

		data.Rooms = append(data.Rooms, room)

	}

	if err := rows.Err(); err != nil {

		rows.Close()

		log.Println("Room Rows Error:", err)

		http.Error(w, "เกิดข้อผิดพลาดในการอ่านห้องพัก", http.StatusInternalServerError)

		return

	}

	rows.Close()

	data.RoomCount = len(data.Rooms)

	// Create a CSRF token for the POST delete forms on this page.
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		log.Println("Room List CSRF Generate Error:", err)
		http.Error(w, "ไม่สามารถสร้างแบบฟอร์มได้", http.StatusInternalServerError)
		return
	}
	data.CSRFToken = hex.EncodeToString(csrfBytes)
	session.Values["landlord_room_delete_csrf"] = data.CSRFToken
	if err := session.Save(r, w); err != nil {
		log.Println("Room List CSRF Save Error:", err)
		http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)
		return
	}
	data.Deleted = r.URL.Query().Get("deleted") == "1"
	if r.URL.Query().Get("delete_error") == "has_requests" {
		data.DeleteError = "ไม่สามารถลบห้องนี้ได้ เพราะมีประวัติคำขอเช่าอยู่"
	}

	// =========================

	// LOAD TEMPLATE

	// =========================

	tmpl, err := template.ParseFiles(

		"templates/landlord_rooms.html",
	)

	if err != nil {

		log.Println("Room List Template Error:", err)

		http.Error(w, "ไม่สามารถโหลดหน้าจัดการห้องพักได้", http.StatusInternalServerError)

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
		http.Error(w, "เกิดข้อผิดพลาดในการตรวจสอบสิทธิ์", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "รหัสห้องพักไม่ถูกต้อง", http.StatusBadRequest)
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
		http.Error(w, "ไม่สามารถโหลดข้อมูลห้องพักได้", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Println("Room Edit CSRF Generation Error:", err)
			http.Error(w, "ไม่สามารถเปิดฟอร์มแก้ไขได้", http.StatusInternalServerError)
			return
		}
		csrfToken := hex.EncodeToString(tokenBytes)
		session.Values["landlord_room_edit_csrf"] = csrfToken
		if err := session.Save(r, w); err != nil {
			log.Println("Room Edit CSRF Save Error:", err)
			http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)
			return
		}
		renderLandlordRoomEdit(w, landlordRoomEditData{Room: room, CSRFToken: csrfToken})
		return
	}

	// POST: limit payload size and validate the CSRF token.
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้องหรือมีขนาดใหญ่เกินไป", http.StatusBadRequest)
		return
	}
	savedToken, ok := session.Values["landlord_room_edit_csrf"].(string)
	submittedToken := r.PostFormValue("csrf_token")
	if !ok || savedToken == "" ||
		subtle.ConstantTimeCompare([]byte(savedToken), []byte(submittedToken)) != 1 {
		http.Error(w, "403 Forbidden: CSRF Token ไม่ถูกต้อง กรุณาเปิดฟอร์มใหม่", http.StatusForbidden)
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
		showError("กรุณากรอกเลขห้องไม่เกิน 20 ตัวอักษร")
		return
	}
	if room.RoomType != "air" && room.RoomType != "fan" {
		showError("กรุณาเลือกประเภทห้องให้ถูกต้อง")
		return
	}
	if room.Status != "available" && room.Status != "occupied" && room.Status != "maintenance" {
		showError("กรุณาเลือกสถานะห้องให้ถูกต้อง")
		return
	}
	if priceErr != nil || math.IsNaN(price) || math.IsInf(price, 0) ||
		price <= 0 || price > 99999999.99 ||
		math.Abs(price*100-math.Round(price*100)) > 0.00001 {
		showError("กรุณากรอกราคาให้ถูกต้อง (มากกว่า 0 และทศนิยมไม่เกิน 2 ตำแหน่ง)")
		return
	}
	if utf8.RuneCountInString(room.Description) > 5000 {
		showError("รายละเอียดห้องยาวเกินกำหนด")
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
		http.Error(w, "ไม่สามารถตรวจสอบเลขห้องได้", http.StatusInternalServerError)
		return
	}
	if duplicates != 0 {
		showError("เลขห้องนี้มีอยู่แล้วในหอพักเดียวกัน")
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
		showError("บันทึกไม่สำเร็จ อาจมีเลขห้องซ้ำ กรุณาตรวจสอบข้อมูลแล้วลองอีกครั้ง")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("Room Edit RowsAffected Error:", err)
		http.Error(w, "ไม่สามารถตรวจสอบผลการบันทึกได้", http.StatusInternalServerError)
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
			http.Error(w, "ไม่สามารถตรวจสอบผลการบันทึกได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดหน้าแก้ไขห้องพักได้", http.StatusInternalServerError)
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
		http.Error(w, "เกิดข้อผิดพลาดของระบบ", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้อง", http.StatusBadRequest)
		return
	}

	csrf, ok := session.Values["landlord_room_delete_csrf"].(string)
	provided := r.PostFormValue("csrf_token")
	if !ok || csrf == "" || subtle.ConstantTimeCompare([]byte(csrf), []byte(provided)) != 1 {
		http.Error(w, "403 Forbidden: CSRF Token ไม่ถูกต้อง กรุณาเปิดรายการห้องพักใหม่", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.PostFormValue("room_id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "รหัสห้องพักไม่ถูกต้อง", http.StatusBadRequest)
		return
	}

	// Lock the room row while checking for rental requests. This protects
	// existing request history from the rooms -> rental_requests CASCADE FK.
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Room Delete BeginTx Error:", err)
		http.Error(w, "ไม่สามารถเริ่มลบห้องพักได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบสิทธิ์ห้องพักได้", http.StatusInternalServerError)
		return
	}

	var requestCount int
	err = tx.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM rental_requests WHERE room_id = ?", roomID,
	).Scan(&requestCount)
	if err != nil {
		log.Println("Room Delete Request Check Error:", err)
		http.Error(w, "ไม่สามารถตรวจสอบประวัติคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ลบห้องพักไม่สำเร็จ", http.StatusInternalServerError)
		return
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		log.Println("Room Delete RowsAffected Error:", err, "rows:", count)
		http.Error(w, "ไม่สามารถยืนยันการลบห้องพักได้", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Println("Room Delete Commit Error:", err)
		http.Error(w, "ไม่สามารถบันทึกการลบห้องพักได้", http.StatusInternalServerError)
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
		http.Error(w, "รหัสหอพักไม่ถูกต้อง", http.StatusBadRequest)
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
		http.Error(w, "ไม่สามารถโหลดข้อมูลหอพักได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดห้องพักได้", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		var room DormDetailRoom
		if err = rows.Scan(&room.ID, &room.RoomNumber, &room.RoomType,
			&room.Price, &room.Description); err != nil {
			rows.Close()
			log.Println("Dorm Detail Room Scan Error:", err)
			http.Error(w, "ไม่สามารถอ่านข้อมูลห้องพักได้", http.StatusInternalServerError)
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
		http.Error(w, "เกิดข้อผิดพลาดในการอ่านห้องพัก", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดคะแนนรีวิวได้", http.StatusInternalServerError)
		return
	}
	reviewRows, err := db.QueryContext(r.Context(), `
        SELECT rev.id, u.name, rev.rating, COALESCE(rev.comment,''), rev.created_at
        FROM reviews rev JOIN users u ON u.id = rev.user_id
        WHERE rev.dormitory_id = ? ORDER BY rev.created_at DESC, rev.id DESC LIMIT 100
    `, dormID)
	if err != nil {
		log.Println("Dorm Reviews Query Error:", err)
		http.Error(w, "ไม่สามารถโหลดรีวิวได้", http.StatusInternalServerError)
		return
	}
	for reviewRows.Next() {
		var item DormReviewItem
		var created time.Time
		if err := reviewRows.Scan(&item.ID, &item.Name, &item.Rating, &item.Comment, &created); err != nil {
			reviewRows.Close()
			log.Println("Dorm Reviews Scan Error:", err)
			http.Error(w, "อ่านรีวิวไม่สำเร็จ", http.StatusInternalServerError)
			return
		}
		item.CreatedAt = created.Format("02/01/2006 15:04")
		page.Reviews = append(page.Reviews, item)
	}
	if err := reviewRows.Err(); err != nil {
		reviewRows.Close()
		log.Println("Dorm Reviews Rows Error:", err)
		http.Error(w, "อ่านรีวิวไม่สำเร็จ", http.StatusInternalServerError)
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
				http.Error(w, "ไม่สามารถเปิดแบบฟอร์มได้", http.StatusInternalServerError)
				return
			}
			var favoriteCount int
			if err := db.QueryRowContext(r.Context(),
				"SELECT COUNT(*) FROM favorites WHERE user_id = ? AND dormitory_id = ?",
				currentUserID, dormID).Scan(&favoriteCount); err != nil {
				log.Println("Dorm Favorite Status Error:", err)
				http.Error(w, "ไม่สามารถโหลดรายการโปรดได้", http.StatusInternalServerError)
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
				http.Error(w, "ไม่สามารถโหลดรีวิวของคุณได้", http.StatusInternalServerError)
				return
			}
			page.HasMyReview = err == nil
		}
	}
	if r.URL.Query().Get("review") == "saved" {
		page.Notice = "บันทึกรีวิวสำเร็จแล้ว"
	}
	if r.URL.Query().Get("favorite") == "add" {
		page.Notice = "เพิ่มหอพักในรายการโปรดแล้ว"
	} else if r.URL.Query().Get("favorite") == "remove" {
		page.Notice = "นำหอพักออกจากรายการโปรดแล้ว"
	}

	tmpl, err := template.ParseFiles("templates/dorm_detail.html")
	if err != nil {
		log.Println("Dorm Detail Template Error:", err)
		http.Error(w, "ไม่สามารถโหลดหน้ารายละเอียดหอพักได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดแบบฟอร์มคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบข้อมูลผู้ใช้ได้", http.StatusInternalServerError)
		return
	}
	if userRole != "user" {
		http.Error(w, "เฉพาะบัญชีผู้เช่าเท่านั้นที่ส่งคำขอเช่าได้", http.StatusForbidden)
		return
	}

	roomID, err := strconv.Atoi(r.URL.Query().Get("room_id"))
	if err != nil || roomID <= 0 {
		http.Error(w, "รหัสห้องพักไม่ถูกต้อง", http.StatusBadRequest)
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
		http.Error(w, "ไม่สามารถโหลดข้อมูลห้องพักได้", http.StatusInternalServerError)
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
			http.Error(w, "ไม่สามารถตรวจสอบคำขอเช่าได้", http.StatusInternalServerError)
			return
		}
		page.HasPending = pendingCount > 0
		page.Success = page.HasPending && r.URL.Query().Get("sent") == "1"
		if status != "available" && !page.HasPending {
			http.Error(w, "ห้องพักนี้ยังไม่เปิดรับคำขอเช่า", http.StatusConflict)
			return
		}
		if !page.HasPending {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				log.Println("Rental Request CSRF Error:", err)
				http.Error(w, "เกิดข้อผิดพลาดในการสร้างฟอร์ม", http.StatusInternalServerError)
				return
			}
			page.CSRFToken = hex.EncodeToString(tokenBytes)
			session.Values["rental_request_csrf"] = page.CSRFToken
			session.Values["rental_request_csrf_room"] = roomID
			if err := session.Save(r, w); err != nil {
				log.Println("Rental Request Session Save Error:", err)
				http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)
				return
			}
		}
		renderRentalRequest(w, page)
		return
	}

	// POST: enforce a small request body and validate CSRF token.
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้องหรือมีขนาดใหญ่เกินไป", http.StatusBadRequest)
		return
	}
	savedToken, tokenOK := session.Values["rental_request_csrf"].(string)
	tokenRoomID, roomOK := session.Values["rental_request_csrf_room"].(int)
	providedToken := r.PostFormValue("csrf_token")
	if !tokenOK || !roomOK || tokenRoomID != roomID || savedToken == "" ||
		subtle.ConstantTimeCompare([]byte(savedToken), []byte(providedToken)) != 1 {
		http.Error(w, "403 Forbidden: กรุณาเปิดฟอร์มคำขอเช่าใหม่", http.StatusForbidden)
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
		formError("กรุณาระบุเบอร์โทรศัพท์ 10 หลัก")
		return
	}
	for _, c := range page.Phone {
		if c < '0' || c > '9' {
			formError("เบอร์โทรศัพท์ต้องเป็นตัวเลขเท่านั้น")
			return
		}
	}
	moveIn, err := time.Parse("2006-01-02", page.MoveInDate)
	if err != nil || moveIn.Format("2006-01-02") != page.MoveInDate || page.MoveInDate < page.Today {
		formError("กรุณาเลือกวันที่ย้ายเข้าตั้งแต่วันนี้เป็นต้นไป")
		return
	}
	if utf8.RuneCountInString(page.Message) > 2000 {
		formError("ข้อความถึงเจ้าของหอต้องไม่เกิน 2,000 ตัวอักษร")
		return
	}

	// Lock the room row to serialize requests, room status changes and deletion.
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Rental Request BeginTx Error:", err)
		http.Error(w, "ไม่สามารถเริ่มบันทึกคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบสถานะห้องได้", http.StatusInternalServerError)
		return
	}
	if actualStatus != "available" {
		formError("ห้องนี้ไม่ว่างแล้ว กรุณาเลือกห้องอื่น")
		return
	}

	var existing int
	err = tx.QueryRowContext(r.Context(), `
        SELECT COUNT(*) FROM rental_requests
        WHERE user_id = ? AND room_id = ? AND status = 'pending'
    `, userID, roomID).Scan(&existing)
	if err != nil {
		log.Println("Rental Request Duplicate Check Error:", err)
		http.Error(w, "ไม่สามารถตรวจสอบคำขอเช่าซ้ำได้", http.StatusInternalServerError)
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
		formError("บันทึกคำขอเช่าไม่สำเร็จ กรุณาลองใหม่")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Println("Rental Request Commit Error:", err)
		http.Error(w, "ไม่สามารถยืนยันการบันทึกคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบบัญชีได้", http.StatusInternalServerError)
		return
	}
	if role != "landlord" {
		http.Error(w, "403 Forbidden: เฉพาะเจ้าของหอพักเท่านั้น", http.StatusForbidden)
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
			http.Error(w, "ไม่สามารถโหลดรายการคำขอเช่าได้", http.StatusInternalServerError)
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
				http.Error(w, "ไม่สามารถอ่านคำขอเช่าได้", http.StatusInternalServerError)
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
			http.Error(w, "ไม่สามารถอ่านรายการคำขอเช่าได้", http.StatusInternalServerError)
			return
		}
		rows.Close()

		// Generate a fresh CSRF token for the review form.
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Println("Landlord Requests CSRF Error:", err)
			http.Error(w, "ไม่สามารถสร้างฟอร์มได้", http.StatusInternalServerError)
			return
		}
		page.CSRFToken = hex.EncodeToString(tokenBytes)
		session.Values["landlord_requests_csrf"] = page.CSRFToken
		if err := session.Save(r, w); err != nil {
			log.Println("Landlord Requests Save Session Error:", err)
			http.Error(w, "ไม่สามารถบันทึก Session ได้", http.StatusInternalServerError)
			return
		}
		tmpl, err := template.ParseFiles("templates/landlord_requests.html")
		if err != nil {
			log.Println("Landlord Requests Template Error:", err)
			http.Error(w, "ไม่สามารถโหลดหน้ารีวิวคำขอเช่าได้", http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, page); err != nil {
			log.Println("Landlord Requests Render Error:", err)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "ข้อมูลฟอร์มไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	savedToken, ok := session.Values["landlord_requests_csrf"].(string)
	submittedToken := r.PostFormValue("csrf_token")
	if !ok || savedToken == "" || subtle.ConstantTimeCompare(
		[]byte(savedToken), []byte(submittedToken),
	) != 1 {
		http.Error(w, "403 Forbidden: กรุณาเปิดหน้าคำขอเช่าใหม่", http.StatusForbidden)
		return
	}
	requestID, err := strconv.Atoi(r.PostFormValue("request_id"))
	if err != nil || requestID <= 0 {
		http.Error(w, "รหัสคำขอเช่าไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	action := r.PostFormValue("action")
	if action != "approve" && action != "reject" {
		http.Error(w, "คำสั่งไม่ถูกต้อง", http.StatusBadRequest)
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
		http.Error(w, "ไม่สามารถตรวจสอบคำขอเช่าได้", http.StatusInternalServerError)
		return
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Println("Landlord Requests BeginTx Error:", err)
		http.Error(w, "ไม่สามารถเริ่มบันทึกคำขอได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบสิทธิ์ได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบสถานะคำขอได้", http.StatusInternalServerError)
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
			http.Error(w, "อนุมัติคำขอไม่สำเร็จ", http.StatusInternalServerError)
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			"UPDATE rooms SET status = 'occupied' WHERE id = ? AND status = 'available'", roomID,
		); err != nil {
			log.Println("Landlord Requests Occupy Room Error:", err)
			http.Error(w, "ไม่สามารถเปลี่ยนสถานะห้องได้", http.StatusInternalServerError)
			return
		}
		// Other pending requests for the same room cannot be fulfilled.
		if _, err = tx.ExecContext(r.Context(), `
            UPDATE rental_requests SET status = 'rejected'
            WHERE room_id = ? AND id <> ? AND status = 'pending'
        `, roomID, requestID); err != nil {
			log.Println("Landlord Requests Reject Others Error:", err)
			http.Error(w, "ไม่สามารถปิดคำขออื่นได้", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err = tx.ExecContext(r.Context(),
			"UPDATE rental_requests SET status = 'rejected' WHERE id = ? AND status = 'pending'", requestID,
		); err != nil {
			log.Println("Landlord Requests Reject Error:", err)
			http.Error(w, "ปฏิเสธคำขอไม่สำเร็จ", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Println("Landlord Requests Commit Error:", err)
		http.Error(w, "ไม่สามารถยืนยันการเปลี่ยนแปลงได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบข้อมูลผู้ใช้ได้", http.StatusInternalServerError)
		return
	}
	if role != "user" {
		http.Error(w, "403 Forbidden: หน้านี้สำหรับผู้เช่าห้องพักเท่านั้น", http.StatusForbidden)
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
		http.Error(w, "ไม่สามารถโหลดคำขอเช่าได้", http.StatusInternalServerError)
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
			http.Error(w, "ไม่สามารถอ่านข้อมูลคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดคำขอเช่าได้", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("templates/my_requests.html")
	if err != nil {
		log.Println("My Requests Template Error:", err)
		http.Error(w, "ไม่สามารถโหลดหน้าคำขอเช่าได้", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถตรวจสอบบัญชีผู้ใช้ได้", http.StatusInternalServerError)
		return 0, "", nil, false
	}
	if role != "user" {
		http.Error(w, "403 Forbidden: สำหรับบัญชีผู้เช่าเท่านั้น", http.StatusForbidden)
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
		http.Error(w, "ข้อมูลแบบฟอร์มไม่ถูกต้อง", http.StatusBadRequest)
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
		http.Error(w, "403 Forbidden: CSRF token ไม่ถูกต้อง กรุณาเปิดหน้าใหม่", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "รหัสหอพักไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	var exists int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM dormitories WHERE id = ?", dormID).Scan(&exists); err != nil {
		log.Println("Favorite Dorm Check Error:", err)
		http.Error(w, "ไม่สามารถตรวจสอบหอพักได้", http.StatusInternalServerError)
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
		http.Error(w, "คำสั่งไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println("Favorite Change Error:", err)
		http.Error(w, "บันทึกรายการโปรดไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "403 Forbidden: CSRF token ไม่ถูกต้อง กรุณาเปิดหน้าใหม่", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "รหัสหอพักไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	rating, err := strconv.Atoi(r.PostFormValue("rating"))
	if err != nil || rating < 1 || rating > 5 {
		http.Error(w, "กรุณาให้คะแนนตั้งแต่ 1 ถึง 5 ดาว", http.StatusBadRequest)
		return
	}
	comment := strings.TrimSpace(r.PostFormValue("comment"))
	if utf8.RuneCountInString(comment) > 1000 {
		http.Error(w, "รีวิวต้องไม่เกิน 1000 ตัวอักษร", http.StatusBadRequest)
		return
	}
	var exists int
	if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM dormitories WHERE id = ?", dormID).Scan(&exists); err != nil {
		log.Println("Review Dorm Check Error:", err)
		http.Error(w, "ตรวจสอบหอพักไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "บันทึกรีวิวไม่สำเร็จ กรุณาตรวจว่าติดตั้ง SQL ของ Step 12 แล้ว", http.StatusInternalServerError)
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
		http.Error(w, "403 Forbidden: CSRF token ไม่ถูกต้อง", http.StatusForbidden)
		return
	}
	dormID, valid := step12DormID(r)
	if !valid {
		http.Error(w, "รหัสหอพักไม่ถูกต้อง", http.StatusBadRequest)
		return
	}
	if _, err := db.ExecContext(r.Context(),
		"DELETE FROM reviews WHERE user_id = ? AND dormitory_id = ?", userID, dormID); err != nil {
		log.Println("Review Delete Error:", err)
		http.Error(w, "ลบรีวิวไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "สร้างฟอร์มไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดรายการโปรดได้", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item FavoriteDormItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Address, &item.Image,
			&item.AvailableRooms, &item.MinPrice); err != nil {
			log.Println("Favorites Scan Error:", err)
			http.Error(w, "ไม่สามารถอ่านรายการโปรดได้", http.StatusInternalServerError)
			return
		}
		page.Dorms = append(page.Dorms, item)
	}
	if err := rows.Err(); err != nil {
		log.Println("Favorites Rows Error:", err)
		http.Error(w, "ไม่สามารถอ่านรายการโปรดได้", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.ParseFiles("templates/favorites.html")
	if err != nil {
		log.Println("Favorites Template Error:", err)
		http.Error(w, "โหลดหน้ารายการโปรดไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "สร้างฟอร์มไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "ไม่สามารถโหลดรีวิวได้", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item MyReviewItem
		var created time.Time
		if err := rows.Scan(&item.DormID, &item.DormName, &item.Rating, &item.Comment, &created); err != nil {
			log.Println("My Reviews Scan Error:", err)
			http.Error(w, "อ่านข้อมูลรีวิวไม่สำเร็จ", http.StatusInternalServerError)
			return
		}
		item.CreatedAt = created.Format("02/01/2006 15:04")
		page.Reviews = append(page.Reviews, item)
	}
	if err := rows.Err(); err != nil {
		log.Println("My Reviews Rows Error:", err)
		http.Error(w, "อ่านข้อมูลรีวิวไม่สำเร็จ", http.StatusInternalServerError)
		return
	}
	tmpl, err := template.ParseFiles("templates/my_reviews.html")
	if err != nil {
		log.Println("My Reviews Template Error:", err)
		http.Error(w, "โหลดหน้ารีวิวไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "เกิดข้อผิดพลาดในการตรวจสอบสิทธิ์", http.StatusInternalServerError)
		return 0, "", nil, false
	}
	if role != "admin" {
		http.Error(w, "403 Forbidden: สำหรับผู้ดูแลระบบเท่านั้น", http.StatusForbidden)
		return 0, "", nil, false
	}
	return id, name, session, true
}

func adminRender(w http.ResponseWriter, filename string, data any) {
	t, err := template.ParseFiles("templates/" + filename)
	if err != nil {
		log.Println("Admin template parse error:", err)
		http.Error(w, "โหลดหน้า Admin ไม่สำเร็จ", http.StatusInternalServerError)
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
		http.Error(w, "โหลดสถิติไม่สำเร็จ", 500)
		return
	}
	data.RecentDorms, err = adminReadDorms(r, 5)
	if err != nil {
		log.Println("Admin dorms error:", err)
		http.Error(w, "โหลดข้อมูลหอพักไม่สำเร็จ", 500)
		return
	}
	data.RecentRequests, err = adminReadRequests(r, 5)
	if err != nil {
		log.Println("Admin requests error:", err)
		http.Error(w, "โหลดคำขอเช่าไม่สำเร็จ", 500)
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
			http.Error(w, "ข้อมูลฟอร์มผิดพลาด", 400)
			return
		}
		expected, ok := session.Values["admin_csrf"].(string)
		submitted := r.PostFormValue("csrf_token")
		if !ok || expected == "" || submitted == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) != 1 {
			http.Error(w, "403 Forbidden: CSRF Token ไม่ถูกต้อง กรุณาเปิดหน้าใหม่", 403)
			return
		}
		id, err := strconv.Atoi(r.PostFormValue("user_id"))
		role := r.PostFormValue("role")
		if err != nil || id <= 0 || (role != "user" && role != "landlord") {
			http.Error(w, "ข้อมูลผู้ใช้หรือสิทธิ์ไม่ถูกต้อง", 400)
			return
		}
		if id == actorID {
			http.Error(w, "ไม่อนุญาตให้แก้สิทธิ์ตนเอง", 403)
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
			http.Error(w, "เปลี่ยนสิทธิ์ไม่สำเร็จ", 500)
			return
		}
		affected, err := result.RowsAffected()
		if err != nil {
			log.Println("Admin rows affected error:", err)
			http.Error(w, "ตรวจผลการแก้ไขไม่สำเร็จ", 500)
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
		http.Error(w, "สร้าง CSRF Token ไม่สำเร็จ", 500)
		return
	}
	rows, err := db.QueryContext(r.Context(), `
        SELECT u.id,u.name,u.phone,u.role,DATE_FORMAT(u.created_at,'%d/%m/%Y'),
          (SELECT COUNT(*) FROM dormitories d WHERE d.owner_id=u.id)
        FROM users u ORDER BY u.id DESC LIMIT 200`)
	if err != nil {
		log.Println("Admin users query error:", err)
		http.Error(w, "โหลดผู้ใช้ไม่สำเร็จ", 500)
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
		http.Error(w, "โหลดผู้ใช้ไม่สำเร็จ", 500)
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
		http.Error(w, "โหลดหอพักไม่สำเร็จ", 500)
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
		http.Error(w, "โหลดคำขอเช่าไม่สำเร็จ", 500)
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
		return "", errors.New("เอกสารทดสอบต้องมีขนาดไม่เกิน 3 MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".pdf" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return "", errors.New("รองรับเอกสารตัวอย่าง .txt, .pdf, .png, .jpg เท่านั้น")
	}
	contents, err := io.ReadAll(io.LimitReader(upload, max+1))
	if err != nil || len(contents) == 0 || len(contents) > int(max) {
		return "", errors.New("อ่านไฟล์เอกสารไม่สำเร็จหรือไฟล์ใหญ่เกิน 3 MB")
	}
	switch ext {
	case ".txt":
		if !utf8.Valid(contents) || bytes.IndexByte(contents, 0) >= 0 {
			return "", errors.New("ไฟล์ .txt ต้องเป็นข้อความ UTF-8")
		}
	case ".pdf":
		if !bytes.HasPrefix(contents, []byte("%PDF-")) {
			return "", errors.New("เอกสาร .pdf ไม่ถูกต้อง")
		}
	case ".png":
		if _, _, err := image.DecodeConfig(bytes.NewReader(contents)); err != nil {
			return "", errors.New("ไฟล์ภาพ PNG ไม่ถูกต้อง")
		}
		if !bytes.HasPrefix(contents, []byte("\x89PNG\r\n\x1a\n")) {
			return "", errors.New("รูปแบบไฟล์ PNG ไม่ถูกต้อง")
		}
	case ".jpg", ".jpeg":
		if len(contents) < 3 || !bytes.Equal(contents[:3], []byte{0xff, 0xd8, 0xff}) {
			return "", errors.New("ไฟล์ JPEG ไม่ถูกต้อง")
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(contents)); err != nil {
			return "", errors.New("ไฟล์ JPEG ไม่ถูกต้อง")
		}
	}
	if err := os.MkdirAll(demoDocsDir, 0700); err != nil {
		return "", errors.New("ไม่สามารถสร้างโฟลเดอร์เอกสารได้")
	}
	f, err := os.CreateTemp(demoDocsDir, "proof-*")
	if err != nil {
		return "", errors.New("ไม่สามารถบันทึกเอกสารได้")
	}
	name := f.Name()
	defer func() { f.Close() }()
	if err := f.Chmod(0600); err != nil {
		os.Remove(name)
		return "", errors.New("กำหนดสิทธิ์ไฟล์ไม่สำเร็จ")
	}
	if _, err := f.Write(contents); err != nil {
		os.Remove(name)
		return "", errors.New("เขียนไฟล์เอกสารไม่สำเร็จ")
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", errors.New("ปิดไฟล์เอกสารไม่สำเร็จ")
	}
	target := name + ext
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return "", errors.New("ตั้งชื่อไฟล์เอกสารไม่สำเร็จ")
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
		http.Error(w, "โหลดสถานะไม่สำเร็จ", 500)
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
			http.Error(w, "ข้อมูลไม่ถูกต้อง", 400)
			return
		}
		token, ok := session.Values["admin_csrf"].(string)
		posted := r.PostFormValue("csrf_token")
		if !ok || token == "" || posted == "" || subtle.ConstantTimeCompare([]byte(token), []byte(posted)) != 1 {
			http.Error(w, "CSRF Token ไม่ถูกต้อง", 403)
			return
		}
		appID, err := strconv.Atoi(r.PostFormValue("application_id"))
		decision := r.PostFormValue("decision")
		note := strings.TrimSpace(r.PostFormValue("review_note"))
		if err != nil || appID <= 0 || (decision != "approved" && decision != "rejected") || len([]rune(note)) > 255 {
			http.Error(w, "ข้อมูลการอนุมัติไม่ถูกต้อง", 400)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "เปิดรายการไม่สำเร็จ", 500)
			return
		}
		defer tx.Rollback()
		var targetID int
		var status string
		err = tx.QueryRowContext(r.Context(), "SELECT user_id,status FROM landlord_applications WHERE id=? FOR UPDATE", appID).Scan(&targetID, &status)
		if err == sql.ErrNoRows {
			http.Error(w, "ไม่พบคำขอ", 404)
			return
		}
		if err != nil {
			http.Error(w, "ตรวจคำขอไม่สำเร็จ", 500)
			return
		}
		if status != "pending" {
			http.Redirect(w, r, "/admin/landlord-applications?result=unchanged", 303)
			return
		}
		if targetID == actorID {
			http.Error(w, "ไม่สามารถอนุมัติคำขอของตนเอง", 403)
			return
		}
		if decision == "approved" {
			result, err := tx.ExecContext(r.Context(), "UPDATE users SET role='landlord' WHERE id=? AND role='user'", targetID)
			if err != nil {
				log.Println("Landlord approval user update error:", err)
				http.Error(w, "เปลี่ยนสิทธิ์ไม่สำเร็จ", 500)
				return
			}
			affected, err := result.RowsAffected()
			if err != nil || affected != 1 {
				http.Error(w, "บัญชีไม่อยู่ในสถานะ user จึงอนุมัติไม่ได้", 409)
				return
			}
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE landlord_applications
           SET status=?,review_note=?,reviewed_by=?,reviewed_at=NOW()
           WHERE id=? AND status='pending'`, decision, note, actorID, appID)
		if err != nil {
			log.Println("Admin landlord application update:", err)
			http.Error(w, "บันทึกผลการตรวจไม่สำเร็จ", 500)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "บันทึกผลตรวจไม่สำเร็จ", 500)
			return
		}
		http.Redirect(w, r, "/admin/landlord-applications?result="+decision, 303)
		return
	}
	token, err := adminCSRFToken(w, r, session)
	if err != nil {
		http.Error(w, "สร้าง CSRF Token ไม่สำเร็จ", 500)
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT a.id,u.name,u.phone,a.dorm_name,a.status,
        a.original_filename,DATE_FORMAT(a.created_at,'%d/%m/%Y %H:%i'),
        COALESCE(DATE_FORMAT(a.reviewed_at,'%d/%m/%Y %H:%i'),''),COALESCE(a.review_note,'')
        FROM landlord_applications a JOIN users u ON u.id=a.user_id
        ORDER BY FIELD(a.status,'pending','rejected','approved'),a.id DESC LIMIT 200`)
	if err != nil {
		log.Println("Applications list error:", err)
		http.Error(w, "โหลดคำขอไม่สำเร็จ ตรวจสอบการ Import SQL", 500)
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
		http.Error(w, "โหลดคำขอไม่สำเร็จ", 500)
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
		http.Error(w, "รหัสเอกสารไม่ถูกต้อง", 400)
		return
	}
	var docPath string
	err = db.QueryRowContext(r.Context(), "SELECT document_path FROM landlord_applications WHERE id=?", id).Scan(&docPath)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "โหลดเอกสารไม่สำเร็จ", 500)
		return
	}
	clean := filepath.Clean(filepath.FromSlash(docPath))
	if filepath.Dir(clean) != filepath.Clean(demoDocsDir) || !strings.HasPrefix(filepath.Base(clean), "proof-") {
		http.Error(w, "ตำแหน่งไฟล์ไม่ถูกต้อง", 403)
		return
	}
	f, err := os.Open(clean)
	if err != nil {
		http.Error(w, "ไม่พบไฟล์เอกสาร", 404)
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
