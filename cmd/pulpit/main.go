package main
import ("fmt";"log";"net/http";"os";"github.com/stockyard-dev/stockyard-pulpit/internal/server";"github.com/stockyard-dev/stockyard-pulpit/internal/store")
func main(){port:=os.Getenv("PORT");if port==""{port="9700"};dataDir:=os.Getenv("DATA_DIR");if dataDir==""{dataDir="./pulpit-data"}
db,err:=store.Open(dataDir);if err!=nil{log.Fatalf("pulpit: %v",err)};defer db.Close();srv:=server.New(db)
fmt.Printf("\n  Pulpit — Self-hosted presentation tool\n  Dashboard:  http://localhost:%s/ui\n  API:        http://localhost:%s/api\n\n",port,port)
log.Printf("pulpit: listening on :%s",port);log.Fatal(http.ListenAndServe(":"+port,srv))}
