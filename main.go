package main

import (
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var err error

//Matches columns in table Cars - for the date fields, get a time.Time from string like:
//time1, err := time.Parse("2006-1-2", car1.DatePurchased)
type car struct {
	CarID                int
	LicensePlate         string
	Make                 string
	Model                string
	ModelYear            int
	OdometerReading      float32
	Units                string
	DatePurchased        string
	MileageWhenPurchased float32
	CurrentlyActive      bool
	MileageWhenSold      float64
	DateSold             string
	Nickname             string
}

type fillUp struct {
	PurchaseID       int
	CarID            int
	StationID        int
	PurchaseDate     string
	GallonsPurchased float32
	IsFillUp         bool
	TripMileage      float32
	OdometerReading  float32
	Cost             float32
	Units            string
}

type serviceStation struct {
	StationID int
	Name      string
	Address   string
}

type user struct {
	UserID   int
	Username string
	//TODO: Impliment password hashing so that text passwords are not stored
	Password []byte
}

type repair struct {
	TransactionID   int
	CarID           int
	StationID       int
	PurchaseDate    string
	OdometerReading float32
	Cost            float32
	Description     string
	Units           string
}

type appConfig struct {
	DBConfig   string `json:"dbCon"`
	CertConfig struct {
		Fullchain string `json:"fullchain"`
		PrivKey   string `json:"privkey"`
	} `json:"certconfigs"`
	OAuthConfig struct {
		ClientID     string `json:"clientid"`
		ClientSecret string `json:"clientsecret"`
	} `json:"oauthconfigs"`
}

type oauthUser struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	dbID          int
}

var tpl *template.Template
var config appConfig
var oauthconfig *oauth2.Config
var oauthstate string
var currentUser oauthUser

func init() {
	tpl = template.Must(template.ParseGlob("templates/*.gohtml"))

	file, err := ioutil.ReadFile("secret.config.json")
	if err != nil {
		log.Fatalln("config file error")
	}
	json.Unmarshal(file, &config)

	oauthconfig = &oauth2.Config{
		ClientID:     config.OAuthConfig.ClientID,
		ClientSecret: config.OAuthConfig.ClientSecret,
		RedirectURL:  "https://cardata.jasonradcliffe.com/success",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}
}

func main() {
	fmt.Println("testnumero1: " + oauthconfig.AuthCodeURL("state125"))

	//format needed by sql driver: [username]:[password]@tcp([address:port])/[dbname]?charset=utf8
	db, err = sql.Open("mysql", config.DBConfig)
	check(err)
	defer db.Close()

	//Check the connection to the database - If the credentials are wrong this will err out
	err = db.Ping()
	check(err)

	car1 := getCarFromID(7)
	fmt.Println(car1)

	//Routes-------------------------------------------------------------------
	//---------Unauthenticated pages-------------------------------------------
	http.HandleFunc("/", index)
	http.HandleFunc("/login", login)
	http.HandleFunc("/oauthlogin", oauthlogin)
	http.HandleFunc("/oauthloginalexa", oauthloginAlexa)
	http.HandleFunc("/signup", signup)
	http.HandleFunc("/about", about)
	http.HandleFunc("/ping", ping)
	//---------Authenticated pages---------------------------------------------
	http.HandleFunc("/viewCars", viewAllCars)
	http.HandleFunc("/viewFillUps", viewFillUps)
	http.HandleFunc("/success", success)
	//--------------------------------------------------------End Routes-------

	//Server Setup and Config--------------------------------------------------
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	srv := &http.Server{
		Addr:         ":443",
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}
	//From the config file: the file path to the fullchain .pem and privkey .pem
	log.Fatalln(srv.ListenAndServeTLS(config.CertConfig.Fullchain, config.CertConfig.PrivKey))
	//-----------------------------------------------End Server Setup and Config---
}

func check(err error) {
	if err != nil {
		log.Fatalln("something must have happened: ", err)
	}
}

//Route Functions-----------------------------------------------------------
//--------------Unauthenticated Pages---------------------------------------
func index(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, "homepage")
}

func ping(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, "<h1>PONG</h1>")
	fmt.Println("PING PONG PING PONG PING PONG")
}

func login(res http.ResponseWriter, req *http.Request) {
	//The Login page for the app - contains a "Login with Google" button
	io.WriteString(res, `<a href="/oauthlogin"> Login with Google </a>`)
}

func oauthloginAlexa(res http.ResponseWriter, req *http.Request) {
	//Each time oauthlogin() is called, a unique, random string gets added to the URL for security
	//oauthstate = numGenerator()
	url2 := "https://accounts.google.com/o/oauth2/v2/auth?access_type=offline"
	//url := oauthconfig.AuthCodeURL(oauthstate)
	http.Redirect(res, req, url2, http.StatusTemporaryRedirect)
}

func oauthlogin(res http.ResponseWriter, req *http.Request) {
	//Each time oauthlogin() is called, a unique, random string gets added to the URL for security
	oauthstate = numGenerator()
	url := oauthconfig.AuthCodeURL(oauthstate)
	http.Redirect(res, req, url, http.StatusTemporaryRedirect)
}

func signup(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, "signup page")
	tpl.ExecuteTemplate(res, "signup.gphtml", nil)

}
func about(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, "about page")

}

//-----------Authenticated Pages------------------------------------------
func success(res http.ResponseWriter, req *http.Request) {
	receivedState := req.FormValue("state")

	//Verify that the state parameter is the same coming back from Google as was set when we generated the URL
	if receivedState != oauthstate {
		res.WriteHeader(http.StatusForbidden)
	} else {

		//Use the code that Google returns to exchange for an access token
		code := req.FormValue("code")
		token, err := oauthconfig.Exchange(oauth2.NoContext, code)
		check(err)

		//Use the Access token to access the identity API, and get the user info
		response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
		check(err)
		defer response.Body.Close()

		contents, err := ioutil.ReadAll(response.Body)
		check(err)
		json.Unmarshal(contents, &currentUser)

		if currentUser.VerifiedEmail == false {
			login(res, req)
		} else {
			viewAllCars(res, req)
		}

	}

}

func viewAllCars(res http.ResponseWriter, req *http.Request) {
	fmt.Println("currentUser:", currentUser)

	if currentUser.Email == "" {
		login(res, req)
	} else {
		rows, err := db.Query(`SELECT * FROM Car;`)
		check(err)
		defer rows.Close()

		s := "RETRIEVED RECORDS:\n"

		for rows.Next() {
			var car1 car
			var sMileageWhenSold sql.NullFloat64
			var sDateSold, sNickname sql.NullString

			err = rows.Scan(&car1.CarID, &car1.LicensePlate, &car1.Make, &car1.Model, &car1.ModelYear,
				&car1.OdometerReading, &car1.Units, &car1.DatePurchased, &car1.MileageWhenPurchased,
				&car1.CurrentlyActive, &sMileageWhenSold, &sDateSold, &sNickname)
			check(err)
			//s += "Car1:" + car1

			if sMileageWhenSold.Valid {
				car1.MileageWhenSold = sMileageWhenSold.Float64
			}
			if sDateSold.Valid {
				car1.DateSold = sDateSold.String
			}
			if sNickname.Valid {
				car1.Nickname = sNickname.String
			}

			time1, err := time.Parse("2006-1-2", car1.DatePurchased)
			check(err)
			fmt.Println("Car was purchased in the month of:" + strconv.Itoa(int(time1.Month())))

			s += fmt.Sprint(car1) + "\n"
		}
		fmt.Fprintln(res, s)
	}

}

func viewFillUps(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, "{insert viewCars code here!}")
	rows, err := db.Query(`SELECT LicensePlate FROM Car;`)
	check(err)
	defer rows.Close()

	// data to be used in query
	var s, license string
	s = "RETRIEVED RECORDS:\n"

	// query
	for rows.Next() {
		err = rows.Scan(&license)
		check(err)
		s += license + "\n"
	}
	fmt.Fprintln(res, s)
}

//---------------------------------------------End Route Functions------

func numGenerator() string {
	n := make([]byte, 32)
	rand.Read(n)
	return base64.StdEncoding.EncodeToString(n)
}

func getCarFromID(carID int) car {
	rows, err := db.Query("SELECT * FROM Car WHERE CarID =" + strconv.Itoa(carID) + ";")
	check(err)
	defer rows.Close()

	rows.Next()
	var car1 car
	var sMileageWhenSold sql.NullFloat64
	var sDateSold, sNickname sql.NullString
	err = rows.Scan(&car1.CarID, &car1.LicensePlate, &car1.Make, &car1.Model, &car1.ModelYear,
		&car1.OdometerReading, &car1.Units, &car1.DatePurchased, &car1.MileageWhenPurchased,
		&car1.CurrentlyActive, &sMileageWhenSold, &sDateSold, &sNickname)
	check(err)

	if sMileageWhenSold.Valid {
		car1.MileageWhenSold = sMileageWhenSold.Float64
	}
	if sDateSold.Valid {
		car1.DateSold = sDateSold.String
	}
	if sNickname.Valid {
		car1.Nickname = sNickname.String
	}

	return car1
}
