/**
  @author: Hanhai
  @desc: API密钥数据库管理模块，提供使用SQLite存储和读取API密钥的功能
**/

package config

import (
	"errors"
	"flowsilicon/internal/logger"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // 添加SQLite驱动导入，确保在整个包中可用
)

const (
	// API密钥数据库文件名
	apikeysDBFileName = "apikeys.db"
	// API密钥数据库表名
	apikeysTableName = "apikeys"
)

// EnsureApikeys 确保apikeys表已创建，是InitApiKeysDB的对外接口
// dbPath 是数据库文件的路径，通常为"data/config.db"
func EnsureApikeys(dbPath string) error {
	logger.Info("确保API密钥表存在: %s", dbPath)

	// 初始化API密钥表
	if err := InitApiKeysDB(); err != nil {
		logger.Error("初始化API密钥表失败: %v", err)
		return err
	}

	logger.Info("API密钥表初始化成功")
	return nil
}

// 注意: 这个函数假设数据库连接已经通过InitConfigDB()建立
func InitApiKeysDB() error {
	if db == nil {
		logger.Error("数据库连接未初始化，请先调用InitConfigDB")
		return errors.New("数据库连接未初始化")
	}

	// 创建API密钥表如果不存在 - 新表结构
	query := `CREATE TABLE IF NOT EXISTS ` + apikeysTableName + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		balance REAL NOT NULL,
		last_used INTEGER NOT NULL,
		total_calls INTEGER NOT NULL,
		success_calls INTEGER NOT NULL,
		success_rate REAL NOT NULL,
		consecutive_failures INTEGER NOT NULL,
		disabled BOOLEAN NOT NULL,
		disabled_at INTEGER NOT NULL,
		last_tested INTEGER NOT NULL,
		rpm INTEGER NOT NULL,
		tpm INTEGER NOT NULL,
		score REAL NOT NULL,
		is_delete BOOLEAN NOT NULL,
		is_used BOOLEAN NOT NULL DEFAULT FALSE
	)`
	_, err := db.Exec(query)
	return err
}

// LoadApiKeysFromDB 从数据库加载API密钥
func LoadApiKeysFromDB() error {
	// 确保数据库连接已经初始化
	if db == nil {
		logger.Error("数据库连接未初始化，尝试初始化")
		err := InitConfigDB("")
		if err != nil {
			return fmt.Errorf("初始化数据库连接失败: %w", err)
		}

		// 确保API密钥表存在
		if err := InitApiKeysDB(); err != nil {
			return fmt.Errorf("初始化API密钥表失败: %w", err)
		}
	}

	// 检查数据库连接是否可用
	if err := db.Ping(); err != nil {
		logger.Error("数据库连接不可用: %v", err)
		return fmt.Errorf("数据库连接不可用: %w", err)
	}

	// 确保表已存在
	var tableExists int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", apikeysTableName).Scan(&tableExists)
	if err != nil {
		logger.Error("检查API密钥表存在失败: %v", err)
		return err
	}

	if tableExists == 0 {
		logger.Error("API密钥表不存在，尝试创建")
		if err := InitApiKeysDB(); err != nil {
			return fmt.Errorf("创建API密钥表失败: %w", err)
		}
	}

	// 查询所有密钥，包括被逻辑删除的密钥
	rows, err := db.Query(`SELECT 
		key, balance, last_used, total_calls, success_calls, success_rate, 
		consecutive_failures, disabled, disabled_at, last_tested, rpm, tpm, score, is_delete, is_used 
		FROM ` + apikeysTableName)
	if err != nil {
		// 如果是因为表不存在，尝试重新创建表
		if strings.Contains(err.Error(), "no such table") {
			logger.Error("API密钥表不存在，尝试创建")
			if initErr := InitApiKeysDB(); initErr != nil {
				return fmt.Errorf("创建API密钥表失败: %w", initErr)
			}
			// 创建表后，返回空列表，不是错误
			keysMutex.Lock()
			defer keysMutex.Unlock()
			apiKeys = make([]ApiKey, 0)
			logger.Info("已创建API密钥表，但没有密钥数据")
			return nil
		}
		return fmt.Errorf("查询API密钥失败: %w", err)
	}
	defer rows.Close()

	// 临时存储加载的密钥
	var loadedKeys []ApiKey

	// 处理查询结果
	for rows.Next() {
		var key ApiKey
		if err := rows.Scan(
			&key.Key,
			&key.Balance,
			&key.LastUsed,
			&key.TotalCalls,
			&key.SuccessCalls,
			&key.SuccessRate,
			&key.ConsecutiveFailures,
			&key.Disabled,
			&key.DisabledAt,
			&key.LastTested,
			&key.RequestsPerMinute,
			&key.TokensPerMinute,
			&key.Score,
			&key.Delete,
			&key.IsUsed,
		); err != nil {
			logger.Error("扫描API密钥数据失败: %v", err)
			continue
		}

		// 添加到加载的密钥列表，包括被标记为删除的密钥
		loadedKeys = append(loadedKeys, key)
	}

	// 检查查询过程中是否有错误
	if err := rows.Err(); err != nil {
		return fmt.Errorf("处理API密钥数据时发生错误: %w", err)
	}

	// 更新全局密钥列表
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 分配新的切片
	apiKeys = make([]ApiKey, len(loadedKeys))
	copy(apiKeys, loadedKeys)

	// 初始化每个密钥的运行时数据
	for i := range apiKeys {
		apiKeys[i].RequestsPerMinute = 0
		apiKeys[i].TokensPerMinute = 0
		apiKeys[i].RecentRequests = make([]RequestStats, 0)
	}

	logger.Info("已从数据库加载 %d 个API密钥（包括 %d 个逻辑删除的密钥）",
		len(apiKeys),
		countDeletedKeys(apiKeys))
	return nil
}

// countDeletedKeys 计算被标记为删除的密钥数量
func countDeletedKeys(keys []ApiKey) int {
	count := 0
	for _, key := range keys {
		if key.Delete {
			count++
		}
	}
	return count
}

// SaveApiKeysToDB 将API密钥保存到数据库
func SaveApiKeysToDB() error {
	if db == nil {
		logger.Error("数据库连接未初始化，请先调用InitConfigDB")
		return errors.New("数据库连接未初始化")
	}

	keysMutex.RLock()
	defer keysMutex.RUnlock()

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 清空表
	_, err = tx.Exec("DELETE FROM " + apikeysTableName)
	if err != nil {
		return err
	}

	// 准备插入语句
	stmt, err := tx.Prepare(`INSERT INTO ` + apikeysTableName + ` 
		(key, balance, last_used, total_calls, success_calls, success_rate, 
		consecutive_failures, disabled, disabled_at, last_tested, rpm, tpm, score, is_delete, is_used) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// 插入每个密钥
	count := 0
	for _, key := range apiKeys {
		// 创建密钥的副本，以便修改
		keyCopy := key
		// 清空RecentRequests数组，不需要存储到数据库
		keyCopy.RecentRequests = nil

		// 插入数据库
		_, err = stmt.Exec(
			keyCopy.Key,
			keyCopy.Balance,
			keyCopy.LastUsed,
			keyCopy.TotalCalls,
			keyCopy.SuccessCalls,
			keyCopy.SuccessRate,
			keyCopy.ConsecutiveFailures,
			keyCopy.Disabled,
			keyCopy.DisabledAt,
			keyCopy.LastTested,
			keyCopy.RequestsPerMinute,
			keyCopy.TokensPerMinute,
			keyCopy.Score,
			keyCopy.Delete,
			keyCopy.IsUsed,
		)
		if err != nil {
			logger.Error("插入API密钥失败: %v", err)
			return err
		}
		count++
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return err
	}

	logger.Info("已保存 %d 个API密钥到数据库", count)
	return nil
}

// AddApiKeyToDB 将一个新的API密钥添加到数据库
func AddApiKeyToDB(key ApiKey) error {
	if db == nil {
		logger.Error("数据库连接未初始化，请先调用InitConfigDB")
		return errors.New("数据库连接未初始化")
	}

	// 清空RecentRequests数组，不需要存储到数据库
	keyCopy := key
	keyCopy.RecentRequests = nil

	// 插入到数据库
	_, err := db.Exec(`INSERT OR REPLACE INTO `+apikeysTableName+` 
		(key, balance, last_used, total_calls, success_calls, success_rate, 
		consecutive_failures, disabled, disabled_at, last_tested, rpm, tpm, score, is_delete, is_used) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		keyCopy.Key,
		keyCopy.Balance,
		keyCopy.LastUsed,
		keyCopy.TotalCalls,
		keyCopy.SuccessCalls,
		keyCopy.SuccessRate,
		keyCopy.ConsecutiveFailures,
		keyCopy.Disabled,
		keyCopy.DisabledAt,
		keyCopy.LastTested,
		keyCopy.RequestsPerMinute,
		keyCopy.TokensPerMinute,
		keyCopy.Score,
		keyCopy.Delete,
		keyCopy.IsUsed,
	)

	if err != nil {
		logger.Error("添加API密钥到数据库失败: %v", err)
		return err
	}

	logger.Info("已添加API密钥到数据库: %s", MaskKey(key.Key))
	return nil
}
