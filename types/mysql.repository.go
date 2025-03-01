package types

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/natansdj/lets"
	"gorm.io/gorm"
)

type IMySQLRepository interface {
	SetDriver(*gorm.DB, *sync.RWMutex)
}

var (
	ErrNotImplemented = errors.New("not implemented function")
	ErrCantCreate     = errors.New("cant create")
)

type Repository struct {
	DB  *gorm.DB
	Mu  *sync.RWMutex
	Con *sql.DB

	// sync
	DbName string
	Table  string
}

// Implement types.IMySQLRepository.
func (tbl *Repository) SetDriver(db *gorm.DB, mu *sync.RWMutex) {
	tbl.DB = db
	tbl.Mu = mu
	// 	tbl.db = db

	var err error
	tbl.Con, err = db.DB()
	if err != nil {
		lets.LogE(err.Error())
		return
	}
}

// // Create new query to this repository
// func (tbl *Repository) NewQuery(debug bool) (q *gorm.DB) {
// 	q = tbl.DB

// 	if debug {
// 		q = q.Session(&gorm.Session{
// 			Logger: logger.Default.LogMode(logger.Info),
// 		})
// 	} else {
// 		q = q.Session(&gorm.Session{
// 			Logger: logger.Default.LogMode(logger.Silent),
// 		})
// 	}

// 	q = q.Table(tbl.Table)

// 	return q
// }

// Get database name of this repository
func (tbl *Repository) DatabaseName() string {
	return tbl.DbName
}

// Get table name of this repository
func (tbl *Repository) TableName() string {
	return tbl.Table
}

// func (*Repository) Copy(dst interface{}, src interface{}) (err error) {
// 	err = copier.CopyWithOption(dst, src, copier.Option{IgnoreEmpty: true})

// 	src = nil
// 	debug.FreeOSMemory()

// 	return
// }

// func (Repository) Get(query *gorm.DB, dest interface{}) (rows int64, err error) {
// 	tx := query.Find(dest)
// 	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
// 		err = tx.Error
// 		return
// 	}

// 	rows = tx.RowsAffected
// 	return
// }

// func (Repository) First(query *gorm.DB, dest interface{}) (rows int64, err error) {
// 	tx := query.First(dest)
// 	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
// 		err = tx.Error
// 		return
// 	}

// 	rows = tx.RowsAffected
// 	return
// }

// func (Repository) Last(query *gorm.DB, dest interface{}) (rows int64, err error) {
// 	tx := query.Last(dest)
// 	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
// 		err = tx.Error
// 		return
// 	}

// 	rows = tx.RowsAffected
// 	return
// }

// func (Repository) Index(repo SyncRepository) (err error) {

// 	// Get books of index
// 	var tIndexes []map[string]interface{}
// 	tx := repo.NewQuery(false).Raw(`PRAGMA index_list("` + repo.TableName() + `");`).
// 		Find(&tIndexes)
// 	if tx.Error != nil {
// 		err = tx.Error

// 		lets.LogErr(err)
// 		return
// 	}

// 	// find index not implemented
// 	var newIndexes = map[string][]string{}
// 	var installed bool
// 	if tIndexes != nil {
// 		for mKey, fields := range repo.Model().Indexes() {
// 			installed = false

// 			for _, tIndex := range tIndexes {
// 				tIndexName, ok := tIndex["name"].(*interface{})
// 				if !ok {
// 					continue
// 				}

// 				if *tIndexName == repo.TableName()+"_"+mKey+"_idx" {
// 					installed = true
// 					break
// 				}
// 			}

// 			if !installed {
// 				newIndexes[repo.TableName()+"_"+mKey+"_idx"] = fields
// 			}
// 		}
// 	} else {
// 		for mKey, fields := range repo.Model().Indexes() {
// 			newIndexes[repo.TableName()+"_"+mKey+"_idx"] = fields
// 		}
// 	}

// 	// Remap
// 	var indexQuery []string
// 	for name, indexes := range newIndexes {
// 		indexQuery = []string{} // reset
// 		for _, field := range indexes {
// 			indexQuery = append(indexQuery, fmt.Sprintf(`"%s" COLLATE BINARY ASC`, field))
// 		}

// 		q := fmt.Sprintf(`CREATE INDEX "%s" ON "%s" (%s);`, name, repo.TableName(), strings.Join(indexQuery, ", "))
// 		tx := repo.NewQuery(false).Exec(q)
// 		if tx.Error != nil {
// 			err = tx.Error
// 			lets.LogErr(err)
// 			continue
// 		}
// 	}

// 	return
// }

// // Update Bulk
// func (Repository) UpdateBulk(src interface{}) (rows int64, err error) {
// 	err = ErrNotImplemented
// 	return
// }

// type RepositoryMariaDb struct {
// }

// func (*RepositoryMariaDb) FormatTime(t time.Time) interface{} {
// 	return t.Format(time.DateTime)
// }

// type RepositorySqLite struct {
// }

// func (*RepositorySqLite) FormatTime(t time.Time) interface{} {
// 	return t.Unix()
// }
