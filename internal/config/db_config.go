/**
  @author: Hanhai
  @desc:
**/

package config

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flowsilicon/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// 数据库文件名
	dbFileName = "config.db"
	// 数据库表名
	configTableName = "config"
)

var (
	// 数据库实例
	db *sql.DB
)

// InitConfigDB 初始化配置数据库
// dbPath 是数据库文件的路径，如果为空则使用默认路径 data/config.db
func InitConfigDB(dbPath string) error {
	if dbPath == "" {
		// 使用默认路径
		dataDir := "data"
		// 确保目录存在
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return err
		}
		dbPath = filepath.Join(dataDir, dbFileName)
	}

	// 打开数据库连接
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// 设置连接池参数
	db.SetMaxOpenConns(1)                   // 限制最大连接数为1，以减少并发问题
	db.SetMaxIdleConns(1)                   // 最大空闲连接数
	db.SetConnMaxLifetime(30 * time.Minute) // 连接最大生命周期

	// 启用WAL模式和关闭同步模式，提高性能，降低锁定风险
	_, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;")
	if err != nil {
		logger.Warn("设置SQLite PRAGMA失败: %v", err)
		// 继续执行，因为这不是致命错误
	}

	// 测试数据库连接
	if err = db.Ping(); err != nil {
		return err
	}

	// 创建配置表如果不存在
	query := `CREATE TABLE IF NOT EXISTS ` + configTableName + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT NOT NULL
	)`
	_, err = db.Exec(query)
	if err != nil {
		logger.Error("创建配置表失败: %v", err)
		return err
	}

	// 检查表是否确实创建成功
	var tableExists int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", configTableName).Scan(&tableExists)
	if err != nil {
		logger.Error("检查配置表存在失败: %v", err)
		return err
	}

	if tableExists == 0 {
		logger.Error("配置表未创建成功，请检查数据库权限")
		return fmt.Errorf("配置表未创建成功")
	}

	logger.Info("配置表初始化成功")
	return nil
}

// CloseConfigDB 关闭配置数据库
func CloseConfigDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// LoadConfigFromDB 从数据库加载配置
func LoadConfigFromDB() (*Config, error) {
	// 确保数据库连接已经初始化
	if db == nil {
		logger.Error("数据库连接未初始化，尝试初始化")
		err := InitConfigDB("")
		if err != nil {
			return nil, fmt.Errorf("初始化数据库连接失败: %w", err)
		}
	}

	// 检查数据库连接是否可用
	if err := db.Ping(); err != nil {
		logger.Error("数据库连接不可用: %v", err)
		return nil, fmt.Errorf("数据库连接不可用: %w", err)
	}

	var configJSON string
	err := db.QueryRow("SELECT value FROM " + configTableName + " WHERE key = 'config'").Scan(&configJSON)
	if err != nil {
		logger.Error("从数据库获取配置失败: %v", err)

		// // 如果是因为没有找到配置，尝试重新插入默认配置
		// if err == sql.ErrNoRows {
		// 	logger.Info("数据库中没有配置数据，尝试插入默认配置")

		// 	// 使用默认版本号
		// 	version := "v1.3.9"

		// 	// 确保版本已保存
		// 	_, vErr := db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", "version", version)
		// 	if vErr != nil {
		// 		logger.Error("无法插入版本号: %v", vErr)
		// 		// 继续，因为这不是致命错误
		// 	}

		// 	// 创建并保存默认配置
		// 	defaultConfig := fmt.Sprintf(`{
		// 		"Server":{"Port":3016},
		// 		"ApiProxy":{
		// 			"BaseURL":"https://api.siliconflow.cn",
		// 			"ModelIndex":0,
		// 			"Retry":{
		// 				"MaxRetries":2,
		// 				"RetryDelayMs":1000,
		// 				"RetryOnStatusCodes":[500,502,503,504],
		// 				"RetryOnNetworkErrors":true
		// 			}
		// 		},
		// 		"Proxy":{
		// 			"HttpProxy":"",
		// 			"HttpsProxy":"",
		// 			"SocksProxy":"127.0.0.1:10808",
		// 			"ProxyType":"socks5",
		// 			"Enabled":false
		// 		},
		// 		"App":{
		// 			"Title":"流动硅基 FlowSilicon %s",
		// 			"MinBalanceThreshold":0.8,
		// 			"MaxBalanceDisplay":14,
		// 			"ItemsPerPage":5,
		// 			"MaxStatsEntries":60,
		// 			"RecoveryInterval":10,
		// 			"MaxConsecutiveFailures":5,
		// 			"BalanceWeight":0.4,
		// 			"SuccessRateWeight":0.3,
		// 			"RPMWeight":0.15,
		// 			"TPMWeight":0.15,
		// 			"AutoUpdateInterval":500,
		// 			"StatsRefreshInterval":100,
		// 			"RateRefreshInterval":150,
		// 			"ModelKeyStrategies":{},
		// 			"HideIcon":false
		// 		},
		// 		"Log":{"MaxSizeMB":1, "Level":"warn"}
		// 	}`, version)

		// 	// 检查表是否已创建
		// 	var tableExists int
		// 	tableErr := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", configTableName).Scan(&tableExists)
		// 	if tableErr != nil || tableExists == 0 {
		// 		logger.Error("配置表不存在，尝试创建: %v", tableErr)
		// 		createErr := InitConfigDB("")
		// 		if createErr != nil {
		// 			return nil, fmt.Errorf("创建配置表失败: %w", createErr)
		// 		}
		// 	}

		// 	// 插入默认配置到数据库
		// 	_, err = db.Exec(
		// 		"INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)",
		// 		"config",
		// 		defaultConfig,
		// 	)
		// 	if err != nil {
		// 		logger.Error("插入默认配置失败: %v", err)
		// 		return nil, err
		// 	}

		// 	logger.Info("已成功插入默认配置到数据库")
		// 	configJSON = defaultConfig
		// } else {
		// 	return nil, err
		// }
	}

	var cfg Config
	err = json.Unmarshal([]byte(configJSON), &cfg)
	if err != nil {
		logger.Error("解析配置JSON失败: %v", err)
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 更新全局配置
	config = &cfg
	logger.Info("成功从数据库加载配置")
	return &cfg, nil
}

// SaveConfigToDB 将当前配置保存到数据库
func SaveConfigToDB() error {
	cfg := GetConfig()
	if cfg == nil {
		return nil
	}

	// 将配置转换为JSON
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	// 保存到数据库
	_, err = db.Exec(
		"INSERT OR REPLACE INTO "+configTableName+" (id, key, value) VALUES (1, 'config', ?)",
		string(configJSON),
	)

	if err == nil {
		logger.Info("配置已成功保存到数据库")
	}

	return err
}

// GetVersion 从数据库中获取版本号
// 如果没有找到version记录，返回空字符串
func GetVersion() string {
	if db == nil {
		logger.Error("数据库连接未初始化，请先调用InitConfigDB")
		return ""
	}

	// 查询version配置项
	var version string
	err := db.QueryRow("SELECT value FROM " + configTableName + " WHERE key = 'version'").Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Error("数据库中不存在version配置项")
		} else {
			logger.Error("获取version配置项失败: %v", err)
		}
		return ""
	}

	return version
}

// SaveVersion 保存版本号到数据库
func SaveVersion(version string) error {
	if db == nil {
		return errors.New("数据库连接未初始化，请先调用InitConfigDB")
	}

	// 检查version键是否已存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM " + configTableName + " WHERE key = 'version'").Scan(&count)
	if err != nil {
		return err
	}

	// 根据键是否存在执行插入或更新操作
	if count > 0 {
		// 更新现有记录
		_, err = db.Exec("UPDATE "+configTableName+" SET value = ? WHERE key = 'version'", version)
	} else {
		// 插入新记录
		_, err = db.Exec("INSERT INTO "+configTableName+" (key, value) VALUES ('version', ?)", version)
	}

	if err != nil {
		return err
	}

	logger.Info("版本号 '%s' 已成功保存到数据库", version)
	return nil
}

// DB 返回数据库实例
func DB() *sql.DB {
	return db
}

// ExecWithRetry 执行SQL语句并在遇到数据库锁定错误时进行重试
// operation: 操作名称，用于日志
// maxRetries: 最大重试次数
// stmt: SQL语句
// args: SQL参数
func ExecWithRetry(operation string, maxRetries int, stmt string, args ...interface{}) (sql.Result, error) {
	if db == nil {
		return nil, errors.New("数据库连接未初始化")
	}

	var result sql.Result
	var err error

	for retries := 0; retries < maxRetries; retries++ {
		result, err = db.Exec(stmt, args...)
		if err == nil {
			return result, nil // 执行成功
		}

		// 检查是否是数据库锁定错误
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "SQLITE_BUSY") {
			// 计算递增的等待时间
			waitTime := time.Duration(100*(retries+1)) * time.Millisecond
			logger.Warn("%s遇到数据库锁定，等待%v后重试 (尝试 %d/%d): %v",
				operation, waitTime, retries+1, maxRetries, err)
			time.Sleep(waitTime)
			continue
		}

		// 对于其他类型的错误，直接返回
		break
	}

	return result, err
}
