package main

import (
	"context"
    "errors"
	"fmt"
    "html/template"
	"log"
	"net/http"
	"os"
    "time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var dbpool *sqlitex.Pool

const (
	sqlCreateTable      = "CREATE TABLE mylist (item string, priority string);"
	sqlTables           = "SELECT name FROM sqlite_schema WHERE type ='table' AND name NOT LIKE 'sqlite_%';"
	sqlMyList           = "SELECT item, priority from mylist;"
	sqlMyListByPriority = "SELECT item, priority from mylist ORDER BY priority asc;"
	sqlCountMyItems     = "SELECT COUNT(*) from mylist;"
	sqlInsertMyList     = "INSERT INTO \"mylist\" (item, priority) VALUES ($f1, $f2);"
	sqlNowHere          = "SELECT datetime('now','-1 day','localtime');"
)

var listName string
var startTime string

// Using a Pool to execute SQL in a concurrent HTTP handler.
func main() {
	fmt.Println("render-my-list ---- startup")
	//Get all env variables
	fmt.Println("LOCAL ENVIRONMENT")
	for _, env := range os.Environ() {
		fmt.Printf("%v\n", env)
	}
	fmt.Println()
	fmt.Println("END LOCAL ENVIRONMENT")

    listName = os.Getenv("RENDER_SERVICE_SLUG")
    startTime = time.Now().String()
	var err error
	dbpool, err = sqlitex.Open("file::memory:?cache=shared", 0, 10)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.TODO()
	conn := dbpool.Get(ctx)
	if conn == nil {
		fmt.Println("Unable to create conn from dbpool")
		return
	}
	// Execute a query.
	// Can I inject the "app name" into the table name - like "foo123-mylist"?
	fmt.Println("Initializing new in memory 'mylist' TABLE")

	if err := sqlitex.Execute(conn, sqlCreateTable, nil); err != nil {
		// handle err
		fmt.Println("Error trying to create table")
		fmt.Println(err)
	}

    data, err := executeSql(sqlTables, conn)
    if err != nil {
		// handle err
		fmt.Println("Error trying to list tables")
		fmt.Println(err)
	}
    for _,row := range data {
        for k,v := range row {
            s := fmt.Sprintf("%s:%s ",k,v)
            fmt.Println(s)
            fmt.Println(row)
        
        }
    }
	defer dbpool.Put(conn)

	http.HandleFunc("/", handle)
	http.HandleFunc("/add", handleAdd)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type ItemsUIInfo struct {
    ListName    string
    StartTime   string
    NewItem     Item
    MyList      []map[string]interface{}
}
type Item struct {
    Item        string
    Priority    string
}

func renderUI(w http.ResponseWriter, r *http.Request, newItem Item) {
    t, err := template.ParseFiles("./add.tmpl.html")
    if err != nil {
        fmt.Println(err)
        fmt.Fprint(w, err)
        return
    }
    u := ItemsUIInfo{
        ListName : listName,
        StartTime : startTime,
    }
    emptyItem := Item{}
    if newItem != emptyItem {
        u.NewItem = newItem
    }
    sort := r.FormValue("s")

    conn := dbpool.Get(r.Context())
    if conn == nil {
        return
    }
    sql := sqlMyList
    if sort != "" {
        sql = sqlMyListByPriority
    }
    myList, err := executeSql(sql, conn)
    if err != nil {
        fmt.Println(err)
        fmt.Fprint(w, err)
        return
    }
    u.MyList = myList
    defer dbpool.Put(conn)


    //t.Execute(os.Stdout, u)
    fmt.Println("------------- NewItemUIInfo --------")
    fmt.Printf("%v\n",u)
    fmt.Println("------------- NewItemUIInfo --------")
    t.Execute(w, u)
}


func addItemFromRequest(r *http.Request) (Item, error) {
	i := r.FormValue("i")
	p := r.FormValue("p")
    if i == "" {
        return Item{}, errors.New("item (formvalue 'i') cannot be empty")
    }
    if p == "" {
        p = "DEFAULT_PRIORITY"
    }
    item := Item{
        Item: i,
        Priority: p,
    }
    fmt.Printf("Adding item: +%v\n", item)

    err := addItem(item,r)
    if err != nil {
        fmt.Println(err)
        return item,nil
    }
    return item, nil
}

func addItem(item Item, r *http.Request) error {
	var err error
	conn := dbpool.Get(r.Context())
	if conn == nil {
		return errors.New("Can't get db connection?")
	}
	stmt, err := conn.Prepare(sqlInsertMyList)
	if err != nil {
		fmt.Println(err)
		return err
	}
	stmt.SetText("$f1", item.Item)
	stmt.SetText("$f2", item.Priority)
	hasRow, err := stmt.Step()
	if err != nil {
		fmt.Println(err)
		return err
	}
	if hasRow {
		fmt.Println("hasRow???? IDK ----->>>> ")

	}

	if err := stmt.Finalize(); err != nil {
		fmt.Println(err)
		return err
	}
    fmt.Printf("Added item:%s priority:%s",item.Item , item.Priority)
	defer dbpool.Put(conn)
    return nil
}


func handleAdd(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("handleAdd() userAgent=%s",r.UserAgent())
    item, err := addItemFromRequest(r)
    if err != nil {
        fmt.Println(err)
        fmt.Fprint(w,err)
    }
    renderUI(w,r,item)

}

func handle(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("handle() userAgent=%s",r.UserAgent())
    renderUI(w,r,Item{})
}


func response(conn *sqlite.Conn, w http.ResponseWriter, sort string) {
	err := sqlitex.ExecuteTransient(conn, sqlNowHere, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			appSlug := os.Getenv("RENDER_SERVICE_SLUG")
			messageBack := fmt.Sprintf("LIST:%s:%s\n", appSlug, stmt.ColumnText(0))
			fmt.Println(messageBack)
			fmt.Fprint(w, messageBack)
			sql := sqlMyList
			if sort != "" {
				sql = sqlMyListByPriority
			}
            data, err := executeSql(sql, conn)
            if err != nil {
                fmt.Println(err)
                fmt.Fprintln(w, err)
            }
            for _,row := range data {
                for k,v := range row {
                    s := fmt.Sprintf("%s:%s ",k,v)
                    fmt.Println(s)
                    fmt.Println(row)
                    fmt.Fprint(w,row)
                
                }
            }
			return nil
		},
	})

	if err != nil {
		fmt.Println(err)
		fmt.Fprintln(w, err)
	}
}

func executeSql(sql string, conn *sqlite.Conn) ([]map[string]interface{},error) {

	fmt.Printf("executeSql === sql:%s\n", sql)
	stmt, err := conn.Prepare(sql)
	if err != nil {
		fmt.Println(err)
        return nil,err
	}
    data := []map[string]interface{}{}
    //data := []interface{}{}
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			fmt.Println(err)
            return nil,err
		}
		if !hasRow {
			break
		}
        row := map[string]interface{}{}
		for i := 0; i < stmt.ColumnCount(); i++ {
			colName := stmt.ColumnName(i)
			fmt.Print(colName + ":" + stmt.GetText(colName))
            row[colName] = stmt.GetText(colName)
		}
        data = append(data,row)


	}
    fmt.Println("========== data ========")
    fmt.Printf("%+v\n",data)
    fmt.Println("========== data ========")

    return data,nil
}

