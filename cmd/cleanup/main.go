package main
import ("fmt"; "gorm.io/driver/postgres"; "gorm.io/gorm")
func main() {
	db, _ := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=airconnect port=5432 sslmode=disable"), &gorm.Config{})
	db.Exec("UPDATE devices SET ip_address = '192.168.1.36', api_token = '6c9559c635a8b3af4da87a6036521296dda202b007ea60ec0c69c40a229a01a2', is_online = true WHERE id = '2a902a4f-b673-4bf1-a435-9c1800475040'")
	fmt.Println("DB updated: IP=192.168.1.36")
}
