package main

import (
	"context"
	"database/sql"
	_ "encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	_ "os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/sqltocsv"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	DB_HOST     = "postgres"
	DB_PORT     = "5432"
	DB_USER     = "postgres"
	DB_PASSWORD = "123"
	DB_NAME     = "CanzShopDB"
)

type Product struct {
	Id               string
	Number           string
	Name             string
	Price            string
	Amount           string
	Supplier         Supplier
	Storage          Storage
	Event            Event
	Categories_names string
}

type Supplier struct {
	Id   string
	Name string
}

type Storage struct {
	Id       string
	Address  string
	Capacity string
}

type Event struct {
	Id   string
	Name string
}

type Category struct {
	Id   string
	Name string
}

type User struct {
	Id    int
	Email string
	Role  string
}

type Data struct {
	Products    []Product
	Suppliers   []Supplier
	Storages    []Storage
	Events      []Event
	ProductsA   []Product
	SuppliersA  []Supplier
	StoragesA   []Storage
	EventsA     []Event
	CurrentUser User
	ErrorStr    string
}

var dbinfo = fmt.Sprintf("host = %s port = %s user=%s password=%s dbname=%s sslmode=disable", DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME)
var db, _ = sql.Open("postgres", dbinfo)
var searchStr, sortTable, sortArgument, sortTrait = "", "", "", ""
var filtProduct Product
var filtSupplier Supplier
var filtStorage Storage
var filtEvent Event
var globalError error
var client = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "",
	DB:       0,
})

func main() {
	//Страницы
	http.HandleFunc("/", mainPage)
	http.HandleFunc("/login", login)
	http.HandleFunc("/registration", reg)
	http.HandleFunc("/main", mainPage)
	http.HandleFunc("/admin", admin)
	//Методы сессии
	http.HandleFunc("/refresh", refresh)
	http.HandleFunc("/logout", logout)
	http.HandleFunc("/authorize", authorize)
	//Методы универсальной сортировки
	http.HandleFunc("/sort", sort)
	http.HandleFunc("/search", search)
	//Методы добавления, изменения, удаления и фильтрации данных
	http.HandleFunc("/productCreate", createProduct)
	http.HandleFunc("/productEdit", editProduct)
	http.HandleFunc("/productDelete", deleteProduct)
	http.HandleFunc("/productFilter", filterProduct)
	http.HandleFunc("/supplierCreate", createSupplier)
	http.HandleFunc("/supplierEdit", editSupplier)
	http.HandleFunc("/supplierDelete", deleteSupplier)
	//Методы импорта и экспорта данных
	http.HandleFunc("/export", export)
	//Загрузка bootstrap, модальных окон, изображений и иконок
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	http.Handle("/modals/", http.StripPrefix("/modals", http.FileServer(http.Dir("modals"))))
	http.Handle("/images/", http.StripPrefix("/images", http.FileServer(http.Dir("images"))))
	http.Handle("/icons/", http.StripPrefix("/icons", http.FileServer(http.Dir("icons"))))
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func mainPage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Fprintf(w, "Главная страница")
}

func login(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", dbinfo)
	checkErr(err)
	defer db.Close()
	if r.Method == "GET" {
		c, err := r.Cookie("session_token")
		if err == nil {
			user := client.HGetAll(context.Background(), c.Value).Val()
			if len(user) != 0 {
				exp, _ := time.Parse(time.RFC3339, user["expire"])
				if exp.After(time.Now()) {
					authorize(w, r)
				}
			}
		}
		t, _ := template.ParseFiles("views/login.html")
		t.Execute(w, nil)
	} else {
		rows, err := db.Query("SELECT email, password FROM buyer union SELECT email, password FROM staff;")
		checkErr(err)
		for rows.Next() {
			var email string
			var password string
			err = rows.Scan(&email, &password)
			checkErr(err)
			err = bcrypt.CompareHashAndPassword([]byte(password), []byte(r.FormValue("password")))
			if r.FormValue("email") == email && err == nil {
				fmt.Println("Авторизация успешна")
				sessionToken := uuid.NewString()
				expiresAt := time.Now().Add(120 * time.Minute)
				session := map[string]string{"login": email, "expire": expiresAt.Format(time.RFC3339)}
				for k, v := range session {
					err := client.HSet(context.Background(), sessionToken, k, v).Err()
					checkErr(err)
				}
				cookie := &http.Cookie{
					Name:     "session_token",
					Value:    sessionToken,
					Path:     "/",
					Expires:  expiresAt,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				}
				http.SetCookie(w, cookie)
				http.Redirect(w, r, "/authorize", http.StatusSeeOther)
			}
		}
		t, _ := template.ParseFiles("views/login.html")
		t.Execute(w, template.HTML(`<script>alert('Неверный логин или пароль')</script>`))
	}
}

func reg(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("views/reg.html")
		t.Execute(w, nil)
	} else {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), bcrypt.DefaultCost)
		checkErr(err)
		err = db.QueryRow("INSERT INTO Buyer (Phone, Email, Password) VALUES($1, $2, $3);",
			r.FormValue("phone"), r.FormValue("email"), hashedPassword).Err()
		if err == nil {
			fmt.Println("Регистрация успешна")
			sessionToken := uuid.NewString()
			expiresAt := time.Now().Add(120 * time.Minute)
			session := map[string]string{"login": r.FormValue("email"), "expire": expiresAt.Format(time.RFC3339)}
			for k, v := range session {
				err := client.HSet(context.Background(), sessionToken, k, v).Err()
				checkErr(err)
			}
			cookie := &http.Cookie{
				Name:     "session_token",
				Value:    sessionToken,
				Path:     "/",
				Expires:  expiresAt,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
			}
			http.SetCookie(w, cookie)
			http.Redirect(w, r, "/authorize", http.StatusSeeOther)
		} else {
			t, _ := template.ParseFiles("views/reg.html")
			t.Execute(w, template.HTML(`<script>alert('Ошибка регистрации: `+err.Error()+`')</script>`))
		}
	}
}

func authorize(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	checkErr(err)
	user := client.HGetAll(context.Background(), c.Value).Val()

	rows, err := db.Query("select email from Staff;")
	checkErr(err)
	for rows.Next() {
		var email string
		err = rows.Scan(&email)
		checkErr(err)
		if email == user["login"] {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
	}

	rows, err = db.Query("select email from Buyer;")
	checkErr(err)
	for rows.Next() {
		var email string
		err = rows.Scan(&email)
		checkErr(err)
		if email == user["login"] {
			http.Redirect(w, r, "/main", http.StatusSeeOther)
		}
	}
}

func refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	if err != nil {
		return
	}

	user := client.HGetAll(context.Background(), c.Value).Val()
	if len(user) == 0 {
		return
	}

	exp, _ := time.Parse(time.RFC3339, user["expire"])
	if exp.Before(time.Now()) {
		client.Del(context.Background(), c.Value)
		return
	}

	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(120 * time.Minute)
	session := map[string]string{"login": user["login"], "expire": expiresAt.Format(time.RFC3339)}
	for k, v := range session {
		client.HSet(context.Background(), sessionToken, k, v)
	}
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

func logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	checkErr(err)
	user := client.HGetAll(context.Background(), c.Value).Val()

	if user == nil {
		return
	}
	err = client.Del(context.Background(), c.Value).Err()
	checkErr(err)

	sessionToken := uuid.NewString()
	expiresAt := time.Now().Add(120 * time.Minute)
	session := map[string]string{"login": user["login"], "expire": expiresAt.Format(time.RFC3339)}
	for k, v := range session {
		err := client.HSet(context.Background(), sessionToken, k, v).Err()
		checkErr(err)
	}
	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func admin(w http.ResponseWriter, r *http.Request) {
	refresh(w, r)
	data := getData()
	data.CurrentUser = getCurrentUser(w, r)
	if globalError != nil {
		data.ErrorStr = "(" + string(globalError.(*pq.Error).Code) + ": " + globalError.(*pq.Error).Code.Name() + ") " + globalError.(*pq.Error).Error()
		globalError = nil
	}
	t, err := template.ParseFiles("views/admin.html", "views/header.html", "views/modals.html")
	checkErr(err)
	t.Execute(w, data)
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) User {
	userNew := User{}
	c, err := r.Cookie("session_token")
	if err == nil {
		user := client.HGetAll(context.Background(), c.Value).Val()
		if len(user) != 0 {
			db.QueryRow("select Staff.Id, Staff.Email, Role.Name from Concurrency inner join Role on Role.Id = Concurrency.role_id inner join Staff on Staff.Id = Concurrency.staff_id where Staff.email = $1", user["login"]).Scan(&userNew.Id, &userNew.Email, &userNew.Role)
			if userNew.Id == 0 {
				row := db.QueryRow("select Buyer.Id, Buyer.Email from Buyer where Buyer.email = $1", user["login"])
				row.Scan(&userNew.Id, &userNew.Email)
				userNew.Role = "Клиент"
			}
		}
	}
	return userNew
}

func search(w http.ResponseWriter, r *http.Request) {
	searchStr = r.FormValue("str")
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func sort(w http.ResponseWriter, r *http.Request) {
	sortTable = r.URL.Query().Get("table")
	sortArgument = r.URL.Query().Get("argument")
	sortTrait = r.URL.Query().Get("trait")
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func export(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("table") {
	case "Product":
		rows, _ := db.Query("select * from Product")
		sqltocsv.WriteFile("export/Product.csv", rows)

	case "All":

	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func getData() Data {
	var rows *sql.Rows
	//Продукты
	if sortTable == "Product" {
		rows, _ = db.Query("select Product.Id, Number, Product.Name, Price, Amount, Supplier.Name, Storage.Address, Event.Name, Supplier.Id, Storage.Id, Event.Id, string_agg(Category.Name, ', ') from Product left join Compliance on Compliance.Product_ID = Product.ID left join Category on Compliance.Category_ID = Category.ID inner join Storage on Product.Storage_ID = Storage.ID inner join Event on Product.Event_ID = Event.ID inner join Supplier on Product.Supplier_ID = Supplier.ID group by Product.Id, Number, Product.Name, Price, Amount, Supplier.Name, Storage.Address, Event.Name, Supplier.Id, Storage.Id, Event.Id order by " + sortArgument + " " + sortTrait)
	} else {
		rows, _ = db.Query("select Product.Id, Number, Product.Name, Price, Amount, Supplier.Name, Storage.Address, Event.Name, Supplier.Id, Storage.Id, Event.Id, string_agg(Category.Name, ', ') from Product left join Compliance on Compliance.Product_ID = Product.ID left join Category on Compliance.Category_ID = Category.ID inner join Storage on Product.Storage_ID = Storage.ID inner join Event on Product.Event_ID = Event.ID inner join Supplier on Product.Supplier_ID = Supplier.ID group by Product.Id, Number, Product.Name, Price, Amount, Supplier.Name, Storage.Address, Event.Name, Supplier.Id, Storage.Id, Event.Id")
	}
	data := Data{}
	for rows.Next() {
		product := Product{}
		rows.Scan(&product.Id, &product.Number, &product.Name, &product.Price, &product.Amount, &product.Supplier.Name, &product.Storage.Address, &product.Event.Name, &product.Supplier.Id, &product.Storage.Id, &product.Event.Id, &product.Categories_names)
		data.ProductsA = append(data.ProductsA, product)
		if strings.Contains(product.Name, searchStr) || strings.Contains(product.Number, searchStr) || strings.Contains(product.Id, searchStr) || strings.Contains(product.Price, searchStr) || strings.Contains(product.Amount, searchStr) || strings.Contains(product.Supplier.Name, searchStr) || strings.Contains(product.Storage.Address, searchStr) || strings.Contains(product.Event.Name, searchStr) || strings.Contains(product.Categories_names, searchStr) {
			if strings.Contains(product.Name, filtProduct.Name) && strings.Contains(product.Number, filtProduct.Number) && strings.Contains(product.Id, filtProduct.Id) && strings.Contains(product.Price, filtProduct.Price) && strings.Contains(product.Amount, filtProduct.Amount) && strings.Contains(product.Supplier.Name, filtProduct.Supplier.Name) && strings.Contains(product.Storage.Address, filtProduct.Storage.Address) && strings.Contains(product.Event.Name, filtProduct.Event.Name) && strings.Contains(product.Categories_names, filtProduct.Categories_names) {
				data.Products = append(data.Products, product)
			}
		}
	}
	//Поставщики
	if sortTable == "Supplier" {
		rows, _ = db.Query("select * from Supplier order by " + sortArgument + " " + sortTrait)
	} else {
		rows, _ = db.Query("select * from Supplier")
	}
	for rows.Next() {
		supplier := Supplier{}
		rows.Scan(&supplier.Id, &supplier.Name)
		data.SuppliersA = append(data.SuppliersA, supplier)
		if strings.Contains(supplier.Id, searchStr) || strings.Contains(supplier.Name, searchStr) {
			data.Suppliers = append(data.Suppliers, supplier)
		}
	}
	//Хранилища
	if sortTable == "Storage" {
		rows, _ = db.Query("select * from Storage order by " + sortArgument + " " + sortTrait)
	} else {
		rows, _ = db.Query("select * from Storage")
	}
	for rows.Next() {
		storage := Storage{}
		rows.Scan(&storage.Id, &storage.Address, &storage.Capacity)
		data.StoragesA = append(data.StoragesA, storage)
		if strings.Contains(storage.Id, searchStr) || strings.Contains(storage.Address, searchStr) || strings.Contains(storage.Capacity, searchStr) {
			data.Storages = append(data.Storages, storage)
		}
	}
	//Акции
	if sortTable == "Event" {
		rows, _ = db.Query("select * from Event order by " + sortArgument + " " + sortTrait)
	} else {
		rows, _ = db.Query("select * from Event")
	}
	for rows.Next() {
		event := Event{}
		rows.Scan(&event.Id, &event.Name)
		data.EventsA = append(data.EventsA, event)
		if strings.Contains(event.Id, searchStr) || strings.Contains(event.Name, searchStr) {
			data.Events = append(data.Events, event)
		}
	}
	return data
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("INSERT INTO Product (Number, Name, Price, Amount, Supplier_ID, Storage_ID, Event_ID) VALUES($1, $2, $3, $4, $5, $6, $7);",
		r.FormValue("number"), r.FormValue("name"), r.FormValue("price"), r.FormValue("amount"), r.FormValue("supplier_id"), r.FormValue("storage_id"), r.FormValue("event_id"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func editProduct(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("UPDATE Product SET Number = $2, Name = $3, Price = $4, Amount = $5, Supplier_ID = $6, Storage_ID = $7, Event_ID = $8 WHERE Id = $1;",
		r.FormValue("id"), r.FormValue("number"), r.FormValue("name"), r.FormValue("price"), r.FormValue("amount"), r.FormValue("supplier_id"), r.FormValue("storage_id"), r.FormValue("event_id"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func deleteProduct(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("DELETE FROM Product WHERE Id = $1;", r.FormValue("id"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func filterProduct(w http.ResponseWriter, r *http.Request) {
	filtProduct.Id = r.FormValue("id")
	filtProduct.Number = r.FormValue("number")
	filtProduct.Name = r.FormValue("name")
	filtProduct.Price = r.FormValue("price")
	filtProduct.Amount = r.FormValue("amount")
	filtProduct.Supplier.Name = r.FormValue("supplier")
	filtProduct.Storage.Address = r.FormValue("storage")
	filtProduct.Event.Name = r.FormValue("event")
	filtProduct.Categories_names = r.FormValue("categories")
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func createSupplier(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("INSERT INTO Supplier (Name) VALUES($1);", r.FormValue("name"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func editSupplier(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("UPDATE Supplier SET Name = $2 WHERE Id = $1;", r.FormValue("id"), r.FormValue("name"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func deleteSupplier(w http.ResponseWriter, r *http.Request) {
	_, globalError = db.Exec("DELETE FROM Supplier WHERE Id = $1;", r.FormValue("id"))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
