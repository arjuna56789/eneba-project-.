package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Offer struct {
	ID     int     `json:"id"`
	Seller string  `json:"seller"`
	Price  float64 `json:"price"`
	Rating float64 `json:"rating"`
}
type Product struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Platform string  `json:"platform"`
	Region   string  `json:"region"`
	Genre    string  `json:"genre"`
	Hue      int     `json:"hue"`
	Sold     int     `json:"sold"`
	Discount int     `json:"discount"`
	Price    float64 `json:"price"`
	Offers   []Offer `json:"offers,omitempty"`
}
type Order struct {
	ID    string      `json:"id"`
	Date  string      `json:"date"`
	Total float64     `json:"total"`
	Lines []OrderLine `json:"lines"`
	email string
}
type OrderLine struct {
	Name   string  `json:"name"`
	Seller string  `json:"seller"`
	Price  float64 `json:"price"`
	Key    string  `json:"key"`
}
type User struct{ salt, hash string }

var (
	products []Product
	offerIdx = map[int]*Product{}
	mu       sync.Mutex
	users    = map[string]User{}
	tokens   = map[string]string{}
	orders   []Order
)

func seed() {
	b, err := os.ReadFile("data/products.json")
	if err != nil || json.Unmarshal(b, &products) != nil || len(products) == 0 {
		fmt.Println("dataset not loaded, using generated data:", err)
		products = nil
		seedGenerated()
		return
	}
	oid := 1
	for i := range products {
		for j := range products[i].Offers {
			products[i].Offers[j].ID = oid
			oid++
		}
		products[i].Price = products[i].Offers[0].Price // offers are sorted cheapest first
		offerIdx[products[i].ID] = &products[i]
	}
}

func seedGenerated() {
	names := []string{"Elden Ring", "Cyberpunk 2077", "FIFA 25", "Hogwarts Legacy", "Red Dead Redemption 2", "The Witcher 3", "Minecraft", "Forza Horizon 5", "God of War", "Zelda: Tears of the Kingdom", "Baldur's Gate 3", "Call of Duty: Black Ops 6", "Hades II", "Stardew Valley", "Starfield", "Mario Kart 8"}
	plats := []string{"Steam", "Xbox", "PlayStation", "Nintendo"}
	regs := []string{"Global", "Europe", "United States"}
	genres := []string{"RPG", "Action", "Sports", "Adventure"}
	sellers := []string{"KeyKing", "GameVault", "PixelHub", "DigitalDen"}
	oid := 1
	for i, n := range names {
		p := Product{ID: i + 1, Name: n, Platform: plats[i%4], Region: regs[i%3], Genre: genres[i%4],
			Hue: (i * 47) % 360, Sold: 5000 - i*290, Discount: 10 + (i*7)%45}
		base := 8 + float64((i*13)%45)
		for j := 0; j < 3; j++ {
			p.Offers = append(p.Offers, Offer{oid, sellers[(i+j)%4], float64(int((base*(1+0.07*float64(j)))*100)) / 100, 4.2 + float64((i+j)%8)/10})
			oid++
		}
		p.Price = p.Offers[0].Price
		products = append(products, p)
	}
	for i := range products {
		offerIdx[products[i].ID] = &products[i]
	}
}

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, m string) { js(w, code, map[string]string{"error": m}) }

func lev(a, b string) int {
	d := make([]int, len(b)+1)
	for j := range d {
		d[j] = j
	}
	for i := 1; i <= len(a); i++ {
		prev := d[0]
		d[0] = i
		for j := 1; j <= len(b); j++ {
			t := d[j]
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			d[j] = min(min(d[j]+1, d[j-1]+1), prev+c)
			prev = t
		}
	}
	return d[len(b)]
}

// match: substring, or every query token within edit distance (typo tolerance) of some name word/prefix.
func match(name, q string) bool {
	name, q = strings.ToLower(name), strings.ToLower(strings.TrimSpace(q))
	if q == "" || strings.Contains(name, q) {
		return true
	}
	words := strings.Fields(name)
	for _, t := range strings.Fields(q) {
		ok := false
		for _, w := range words {
			pre := w
			if len(pre) > len(t) {
				pre = pre[:len(t)]
			}
			if strings.Contains(w, t) || lev(w, t) <= len(t)/4 || lev(pre, t) <= len(t)/4 {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	min, _ := strconv.ParseFloat(q.Get("min"), 64)
	max, _ := strconv.ParseFloat(q.Get("max"), 64)
	var out []Product
	for _, p := range products {
		if !match(p.Name, q.Get("q")) || (q.Get("platform") != "" && p.Platform != q.Get("platform")) ||
			(q.Get("region") != "" && p.Region != q.Get("region")) || p.Price < min || (max > 0 && p.Price > max) {
			continue
		}
		p.Offers = nil
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		switch q.Get("sort") {
		case "price_asc":
			return out[i].Price < out[j].Price
		case "price_desc":
			return out[i].Price > out[j].Price
		case "name":
			return out[i].Name < out[j].Name
		}
		return out[i].Sold > out[j].Sold
	})
	js(w, 200, out)
}

// Home blocks: each is an independent endpoint with its own data source.
func bannerSvc(w http.ResponseWriter, _ *http.Request) {
	js(w, 200, []map[string]any{
		{"title": "Black November Sale", "text": "Up to 60% off digital keys", "hue": 270, "link": "#/store?sort=price_asc"},
		{"title": "Elden Ring", "text": "Best price from sellers worldwide", "hue": 10, "link": "#/product/1"},
	})
}
func bestSvc(w http.ResponseWriter, _ *http.Request) {
	c := append([]Product{}, products...)
	sort.Slice(c, func(i, j int) bool { return c[i].Sold > c[j].Sold })
	for i := range c {
		c[i].Offers = nil
	}
	js(w, 200, c[:8])
}
func dealsSvc(w http.ResponseWriter, _ *http.Request) {
	c := append([]Product{}, products...)
	sort.Slice(c, func(i, j int) bool { return c[i].Discount > c[j].Discount })
	for i := range c {
		c[i].Offers = nil
	}
	js(w, 200, c[:8])
}

func product(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if p, ok := offerIdx[id]; ok {
		js(w, 200, p)
		return
	}
	fail(w, 404, "not found")
}

func hash(salt, pw string) string { s := sha256.Sum256([]byte(salt + pw)); return hex.EncodeToString(s[:]) }
func rnd(n int) string          { b := make([]byte, n); rand.Read(b); return strings.ToUpper(hex.EncodeToString(b)) }

func creds(r *http.Request) (string, string, bool) {
	var b struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&b)
	return strings.ToLower(strings.TrimSpace(b.Email)), b.Password, strings.Contains(b.Email, "@") && len(b.Password) >= 6
}
func register(w http.ResponseWriter, r *http.Request) {
	e, p, ok := creds(r)
	if !ok {
		fail(w, 400, "Valid email and 6+ char password required")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ex := users[e]; ex {
		fail(w, 409, "Email already registered")
		return
	}
	s := rnd(8)
	users[e] = User{s, hash(s, p)}
	t := rnd(16)
	tokens[t] = e
	js(w, 200, map[string]string{"token": t, "email": e})
}
func login(w http.ResponseWriter, r *http.Request) {
	e, p, _ := creds(r)
	mu.Lock()
	defer mu.Unlock()
	u, ok := users[e]
	if !ok || u.hash != hash(u.salt, p) {
		fail(w, 401, "Wrong email or password")
		return
	}
	t := rnd(16)
	tokens[t] = e
	js(w, 200, map[string]string{"token": t, "email": e})
}
func authed(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		e, ok := tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		mu.Unlock()
		if !ok {
			fail(w, 401, "Sign in required")
			return
		}
		h(w, r, e)
	}
}

func checkout(w http.ResponseWriter, r *http.Request, email string) {
	var b struct {
		Card  string `json:"card"`
		Items []struct{ OfferID, Qty int }
	}
	json.NewDecoder(r.Body).Decode(&b)
	if len(b.Items) == 0 {
		fail(w, 400, "Cart is empty")
		return
	}
	if strings.ReplaceAll(b.Card, " ", "") != "4242424242424242" { // simulated payment: only the test card succeeds
		fail(w, 402, "Payment declined (use test card 4242 4242 4242 4242)")
		return
	}
	o := Order{ID: "ENB-" + rnd(4), Date: time.Now().Format("2006-01-02 15:04"), email: email}
	for _, it := range b.Items {
		for _, p := range products {
			for _, of := range p.Offers {
				if of.ID == it.OfferID {
					for n := 0; n < max(1, min(it.Qty, 10)); n++ {
						o.Lines = append(o.Lines, OrderLine{p.Name, of.Seller, of.Price, fmt.Sprintf("%s-%s-%s", rnd(2), rnd(2), rnd(2))})
						o.Total += of.Price
					}
				}
			}
		}
	}
	if len(o.Lines) == 0 {
		fail(w, 400, "No valid items")
		return
	}
	mu.Lock()
	orders = append(orders, o)
	mu.Unlock()
	js(w, 200, o)
}
func myOrders(w http.ResponseWriter, _ *http.Request, email string) {
	mu.Lock()
	defer mu.Unlock()
	out := []Order{}
	for _, o := range orders {
		if o.email == email {
			out = append(out, o)
		}
	}
	js(w, 200, out)
}

func main() {
	seed()
	m := http.NewServeMux()
	m.HandleFunc("GET /api/products", list)
	m.HandleFunc("GET /api/products/{id}", product)
	m.HandleFunc("GET /api/home/banner", bannerSvc)
	m.HandleFunc("GET /api/home/bestsellers", bestSvc)
	m.HandleFunc("GET /api/home/deals", dealsSvc)
	m.HandleFunc("POST /api/auth/register", register)
	m.HandleFunc("POST /api/auth/login", login)
	m.HandleFunc("POST /api/checkout", authed(checkout))
	m.HandleFunc("GET /api/orders", authed(myOrders))
	m.Handle("/", http.FileServer(http.Dir("web")))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("listening on :" + port)
	http.ListenAndServe(":"+port, m)
}
