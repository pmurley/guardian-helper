package db

import (
	"errors"
	"fmt"
	"os"

	"database/sql"

	_ "github.com/lib/pq" // Only want to import the interface here
)

type LookupDB struct {
	Database         *sql.DB
	HashFromNameStmt *sql.Stmt
	NameFromHashStmt *sql.Stmt
}

var db1 *LookupDB

const (
	// UnknownClassTable is the name of the table that will hold all the unknown class values provided by Alexa
	UnknownClassTable = "unknown_classes"
	// UnknownItemTable is the name of the table that will hold the unknown item name values passed by Alexa
	UnknownItemTable = "unknown_items"
)

// InitDatabase is in charge of preparing any Statements that will be commonly used as well
// as setting up the database connection pool.
func InitDatabase() error {

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Println("DB errror: ", err.Error())
		return err
	}

	stmt, err := db.Prepare("SELECT item_hash FROM items WHERE item_name = $1 AND item_type_name NOT IN ('Material Exchange', '') ORDER BY max_stack_size DESC LIMIT 1")
	if err != nil {
		fmt.Println("DB error: ", err.Error())
		return err
	}
	nameFromHashStmt, err := db.Prepare("SELECT item_name FROM items WHERE item_hash = $1 LIMIT 1")
	if err != nil {
		fmt.Println("DB prepare error: ", err.Error())
		return err
	}

	db1 = &LookupDB{
		Database:         db,
		HashFromNameStmt: stmt,
		NameFromHashStmt: nameFromHashStmt,
	}

	return nil
}

// GetDBConnection is a helper for getting a connection to the DB based on
// environment variables or some other method.
func GetDBConnection() (*LookupDB, error) {

	if db1 == nil {
		fmt.Println("Initializing db!")
		err := InitDatabase()
		if err != nil {
			fmt.Println("Failed to initialize the database: ", err.Error())
			return nil, err
		}
	}

	return db1, nil
}

// GetItemHashFromName is in charge of querying the database and reading
// the item hash value for the given item name.
func GetItemHashFromName(itemName string) (uint, error) {

	db, err := GetDBConnection()
	if err != nil {
		return 0, err
	}

	row := db.HashFromNameStmt.QueryRow(itemName)

	var hash uint
	err = row.Scan(&hash)

	if err == sql.ErrNoRows {
		fmt.Println("Didn't find any transferrable items with that name: ", itemName)
		InsertUnknownValueIntoTable(itemName, UnknownItemTable)
		return 0, errors.New("No items found")
	} else if err != nil {
		return 0, errors.New(err.Error())
	}

	return hash, nil
}

// GetItemNameFromHash is in charge of querying the database and reading
// the item name value for the given item hash.
func GetItemNameFromHash(itemHash string) (string, error) {

	db, err := GetDBConnection()
	if err != nil {
		return "", err
	}

	row := db.NameFromHashStmt.QueryRow(itemHash)

	var name string
	err = row.Scan(&name)

	if err == sql.ErrNoRows {
		fmt.Println("Didn't find any transferrable items with that hash: ", itemHash)
		return "", errors.New("No items found")
	} else if err != nil {
		return "", errors.New(err.Error())
	}

	return name, nil
}

// InsertUnknownValueIntoTable is a helper method for inserting a value into the specified table.
// This is used when a value for a slot type is not usable. For example when a class name for a character
// is not a valid Destiny class name.
func InsertUnknownValueIntoTable(value, tableName string) {

	conn, err := GetDBConnection()
	if err != nil {
		return
	}

	conn.Database.Exec("INSERT INTO "+tableName+" (value) VALUES(?)", value)
}
