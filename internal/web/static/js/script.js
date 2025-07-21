/**
 @author: AI
 @since: 2025/3/23 22:30:16
 @desc: 主页面功能实现，包含API密钥管理、系统监控、统计分析和用户交互等核心功能
 **/

// 全局变量
let allKeys = [];
let selectedKeyId = null;
let currentPage = 1;
let currentSortField = 'score';
let currentSortDirection = 'desc';

// 保存的密钥模式
let keyMode = 'auto';
let manualSelectedKeys = [];

// 排序相关变量
let selectedKeys = new Set();

// localStorage 存储密钥
const STORAGE_KEY = 'flowsilicon_saved_api_keys';

// 已保存密钥分页相关变量
let savedKeysCurrentPage = 1;

// 延迟常量（毫秒）
const KEY_INFO_DEBOUNCE_DELAY = 300;  // 密钥信息加载防抖延迟
const COPY_BUTTON_RESET_DELAY = 1500; // 复制按钮重置文本延迟
const DOM_CLEANUP_DELAY = 100;        // DOM清理延迟
const API_TEST_INTERVAL = 1000;       // API测试间隔
const TOAST_DISPLAY_DURATION = 1500;  // Toast提示显示时间
const TOAST_ANIMATION_DURATION = 300; // Toast动画持续时间

// 计时器变量
let keysUpdateCountdownTimer = null;  // 密钥列表更新倒计时
let statsUpdateCountdownTimer = null; // 系统概要更新倒计时
let rateUpdateCountdownTimer = null;  // 速率监控更新倒计时
let autoUpdateTimer = null;           // API密钥自动更新定时器
let statsUpdateTimer = null;          // 系统概要自动更新定时器
let rateUpdateTimer = null;           // 速率监控自动更新定时器
let keyInfoDebounceTimer = null;      // 密钥信息加载防抖定时器

// 保存API密钥到本地存储
function saveKeyToLocalStorage(key) {
    let savedKeys = getSavedKeys();
    
    // 检查是否已存在
    if (!savedKeys.includes(key)) {
        savedKeys.push(key);
        localStorage.setItem(STORAGE_KEY, JSON.stringify(savedKeys));
    }
}

// 从本地存储获取保存的API密钥
function getSavedKeys() {
    const savedKeysStr = localStorage.getItem(STORAGE_KEY);
    return savedKeysStr ? JSON.parse(savedKeysStr) : [];
}

// 渲染已保存的API密钥列表
function renderSavedKeysList() {
    const savedKeysList = document.getElementById('saved-keys-list');
    const savedKeys = getSavedKeys();
    
    if (savedKeys.length === 0) {
        savedKeysList.innerHTML = '<p>没有保存的API密钥</p>';
        return;
    }
    
    // 分页变量
    const savedKeysPerPage = 8; // 每页显示10个密钥
    const savedKeysTotalPages = Math.ceil(savedKeys.length / savedKeysPerPage);
    
    // 确保当前页码在有效范围内
    if (savedKeysCurrentPage < 1) savedKeysCurrentPage = 1;
    if (savedKeysCurrentPage > savedKeysTotalPages) savedKeysCurrentPage = savedKeysTotalPages;
    
    // 计算当前页的密钥
    const startIndex = (savedKeysCurrentPage - 1) * savedKeysPerPage;
    const endIndex = Math.min(startIndex + savedKeysPerPage, savedKeys.length);
    const currentPageKeys = savedKeys.slice(startIndex, endIndex);
    
    let html = '<div class="list-group">';
    currentPageKeys.forEach(key => {
        const maskedKey = maskKey(key);
        html += `
            <div class="list-group-item d-flex justify-content-between align-items-center">
                <span>${maskedKey}</span>
                <div>
                    <button class="btn btn-sm btn-primary add-saved-key" data-key="${key}">添加</button>
                    <button class="btn btn-sm btn-danger remove-saved-key" data-key="${key}">删除</button>
                </div>
            </div>
        `;
    });
    html += '</div>';
    
    // 添加分页控件
    if (savedKeysTotalPages > 1) {
        html += '<nav aria-label="已保存密钥分页" class="mt-3"><ul class="pagination pagination-sm justify-content-center" id="saved-keys-pagination">';
        
        // 上一页按钮
        html += `
            <li class="page-item ${savedKeysCurrentPage === 1 ? 'disabled' : ''}">
                <a class="page-link saved-keys-page-link" href="#" data-page="${savedKeysCurrentPage - 1}" aria-label="上一页">
                    <span aria-hidden="true">&laquo;</span>
                </a>
            </li>
        `;
        
        // 页码按钮
        for (let i = 1; i <= savedKeysTotalPages; i++) {
            html += `
                <li class="page-item ${i === savedKeysCurrentPage ? 'active' : ''}">
                    <a class="page-link saved-keys-page-link" href="#" data-page="${i}">${i}</a>
                </li>
            `;
        }
        
        // 下一页按钮
        html += `
            <li class="page-item ${savedKeysCurrentPage === savedKeysTotalPages ? 'disabled' : ''}">
                <a class="page-link saved-keys-page-link" href="#" data-page="${savedKeysCurrentPage + 1}" aria-label="下一页">
                    <span aria-hidden="true">&raquo;</span>
                </a>
            </li>
        `;
        
        html += '</ul></nav>';
    }
    
    savedKeysList.innerHTML = html;
    
    // 添加分页事件
    document.querySelectorAll('.saved-keys-page-link').forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const page = parseInt(this.dataset.page);
            if (page >= 1 && page <= savedKeysTotalPages) {
                savedKeysCurrentPage = page;
                renderSavedKeysList();
            }
        });
    });
    
    // 添加"添加"按钮事件
    document.querySelectorAll('.add-saved-key').forEach(button => {
        button.addEventListener('click', function() {
            const key = this.dataset.key;
            addKey(key, 0); // 余额设为0，系统会自动检查
        });
    });
    
    // 添加"删除"按钮事件
    document.querySelectorAll('.remove-saved-key').forEach(button => {
        button.addEventListener('click', function() {
            const key = this.dataset.key;
            removeSavedKey(key);
            renderSavedKeysList(); // 重新渲染列表
        });
    });
}

// 从本地存储中删除保存的API密钥
function removeSavedKey(key) {
    let savedKeys = getSavedKeys();
    savedKeys = savedKeys.filter(k => k !== key);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(savedKeys));
}

// 清空所有保存的API密钥
function clearSavedKeys() {
    localStorage.removeItem(STORAGE_KEY);
    renderSavedKeysList();
    showToast('已清空所有保存的API密钥', 'info');
}

// 检查是否有有效的密钥（有余额）并更新刷新按钮状态
function updateRefreshButtonState() {
    const refreshButton = document.getElementById('refresh-keys');
    if (!refreshButton) return;
    
    // 检查是否有API密钥
    if (allKeys.length === 0) {
        // 没有密钥，禁用按钮
        refreshButton.disabled = true;
        refreshButton.title = '没有API密钥可刷新';
        return;
    }
    
    // 检查是否有非零余额的密钥
    const hasNonZeroBalance = allKeys.some(key => parseFloat(key.balance) > 0);
    
    if (!hasNonZeroBalance) {
        // 所有密钥余额为零，禁用按钮
        refreshButton.disabled = true;
        refreshButton.title = '所有API密钥余额为零，无需刷新';
    } else {
        // 有非零余额密钥，启用按钮
        refreshButton.disabled = false;
        refreshButton.title = '刷新所有API密钥的余额';
    }
}

// 加载 API 密钥列表
function loadKeys() {
    // 显示刷新按钮的加载状态
    document.getElementById('refresh-spinner').style.display = 'inline-block';
    
    fetch('/keys')
        .then(response => {
            if (!response.ok) {
                return response.json().then(data => {
                    throw new Error(data.error || '加载密钥失败');
                }).catch(error => {
                    throw new Error(`加载密钥失败: ${response.status}`);
                });
            }
            return response.json();
        })
        .then(data => {
            // 获取所有密钥
            allKeys = data.keys || [];
            
            // 获取当前使用的密钥
            keyMode = data.key_mode || 'auto';
            manualSelectedKeys = data.manual_selected_keys || [];
            
            // 添加额外的字段，将balance转为数字以便排序
            allKeys.forEach(key => {
                key.balance_num = parseFloat(key.balance);
                key.score_num = parseFloat(key.score);
                
                // 如果已在selectedKeys中，设置为选中状态
                if (selectedKeys.has(key.key)) {
                    key.selected = true;
                }
            });
            
            // 渲染密钥列表
            renderKeysList();
            
            // 加载当前使用的密钥信息
            loadCurrentKeyInfo();
            
            // 更新刷新按钮状态
            updateRefreshButtonState();
            
            // 检查是否没有密钥
            if (allKeys.length === 0) {
                // 更新最后更新时间，但不启动倒计时
                const now = new Date();
                const timeStr = now.toLocaleTimeString();
                document.getElementById('keys-last-update').textContent = `上次更新: ${timeStr} (已暂停更新)`;
                
                // 清除倒计时计时器
                if (keysUpdateCountdownTimer) {
                    clearInterval(keysUpdateCountdownTimer);
                    keysUpdateCountdownTimer = null;
                }
            } else {
                // 更新最后更新时间并开始倒计时
                startKeysUpdateCountdown(AUTO_UPDATE_INTERVAL);
            }
        })
        .catch(error => {
            console.error('Error loading keys:', error);
            document.getElementById('keys-container').innerHTML = 
                `<div class="alert alert-danger">
                    <strong>加载失败</strong>: ${error.message || '无法连接到服务器，请检查网络连接'}
                </div>`;
            document.getElementById('keys-pagination').innerHTML = '';
            
            // 更新刷新按钮状态 - 没有密钥时禁用
            updateRefreshButtonState();
            
            // 更新最后更新时间，显示已暂停更新
            const now = new Date();
            const timeStr = now.toLocaleTimeString();
            document.getElementById('keys-last-update').textContent = `上次更新: ${timeStr} (已暂停更新)`;
            
            // 清除倒计时计时器
            if (keysUpdateCountdownTimer) {
                clearInterval(keysUpdateCountdownTimer);
                keysUpdateCountdownTimer = null;
            }
        })
        .finally(() => {
            // 隐藏刷新按钮的加载状态
            document.getElementById('refresh-spinner').style.display = 'none';
        });
}

// 开始API密钥列表更新倒计时
function startKeysUpdateCountdown(seconds) {
    if (keysUpdateCountdownTimer) {
        clearInterval(keysUpdateCountdownTimer);
    }
    
    let remainingSeconds = seconds;
    
    // 获取最后更新的时间（当前时间）
    const lastUpdateTime = new Date();
    const lastUpdateTimeStr = lastUpdateTime.toLocaleTimeString();
    
    // 获取元素
    const keysUpdateEl = document.getElementById('keys-last-update');
    if (!keysUpdateEl) {
        return;
    }
    
    // 立即更新显示
    keysUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    
    // 设置新计时器，每秒更新倒计时
    keysUpdateCountdownTimer = setInterval(() => {
        remainingSeconds--;
        
        if (remainingSeconds <= 0) {
            clearInterval(keysUpdateCountdownTimer);
            return;
        }
        
        // 更新倒计时，保持上次更新时间不变
        keysUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    }, 1000);
}

// 渲染密钥列表
function renderKeysList() {
    const keysContainer = document.getElementById('keys-container');
    
    // 获取最低余额阈值，如果没有定义，使用默认值1.0
    const minBalanceThreshold = typeof MIN_BALANCE_THRESHOLD !== 'undefined' ? 
        MIN_BALANCE_THRESHOLD : 1.0;
    
    if (allKeys.length === 0) {
        keysContainer.innerHTML = '<div class="alert alert-info">没有可以API 密钥, 请先添加API密钥</div>';
        return;
    }
    
    // 计算分页
    const totalPages = Math.ceil(allKeys.length / ITEMS_PER_PAGE);
    
    // 确保当前页码在有效范围内
    if (currentPage < 1) currentPage = 1;
    if (currentPage > totalPages) currentPage = totalPages;
    
    // 计算当前页的密钥
    const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
    const endIndex = Math.min(startIndex + ITEMS_PER_PAGE, allKeys.length);
    const currentPageKeys = allKeys.slice(startIndex, endIndex);
    
    let html = '';
    
    currentPageKeys.forEach(key => {
        // 计算余额百分比
        const balancePercent = (key.balance / MAX_BALANCE) * 100;
        
        // 确定余额颜色类
        let balanceClass = '';
        if (balancePercent >= 70) {
            balanceClass = 'balance-high';
        } else if (balancePercent >= 30) {
            balanceClass = 'balance-medium';
        } else {
            balanceClass = 'balance-low';
        }
        
        // 如果密钥被禁用，添加禁用类
        const disabledClass = key.disabled ? 'key-disabled' : '';
        
        // 计算成功率百分比
        const successRatePercent = key.success_rate * 100;
        
        // 掩盖密钥
        const maskedKey = maskKey(key.key);
        
        // 检查是否选中
        const isSelected = key.selected || false;
        const selectedClass = isSelected ? 'selected' : '';

        // console.log("key.score",key.score);
        
        
        // 添加所有需要的数据属性，用于排序功能
        html += `
            <div class="key-item ${balanceClass} ${disabledClass} ${selectedClass}" 
                data-key="${key.key}" 
                data-score="${key.score || 0}" 
                data-balance="${key.balance || 0}" 
                data-success-rate="${key.success_rate || 0}" 
                data-usage="${key.total_calls || 0}" 
                data-rpm="${key.rpm || 0}" 
                data-tpm="${key.tpm || 0}">
                <div class="key-info-row">
                    <div class="key-content">
                        <input type="checkbox" class="form-check-input key-checkbox key-select" data-key="${key.key}" ${key.disabled ? 'disabled' : ''} ${isSelected ? 'checked' : ''}>
                        <span class="key-label ms-2">${maskedKey}</span>
                        <span class="key-score ms-2" data-score="${parseFloat(key.score || 0).toFixed(2)}">${parseFloat(key.score || 0).toFixed(2)}</span>
                        <span class="ms-2">余额: <span class="key-balance ${key.balance < minBalanceThreshold ? 'text-danger' : ''}" data-balance="${key.balance || 0}">${key.balance.toFixed(2)}</span>
                        </span>
                        <span class="key-stat ms-2" data-usage="${key.total_calls || 0}">调用: ${key.total_calls}</span>
                        <span class="key-stat ms-2" data-success-rate="${key.success_rate || 0}">成功率: ${successRatePercent.toFixed(1)}%</span>
                        <span class="key-stat rpm-stat ms-2" data-rpm="${key.rpm || 0}">RPM: <span class="rpm-value">${key.rpm || 0}</span></span>
                        <span class="key-stat tpm-stat ms-2" data-tpm="${key.tpm || 0}">TPM: <span class="tpm-value">${key.tpm || 0}</span></span>
                    </div>
                    <div class="key-actions-container">
                        <div class="api-buttons-container">
                            <div class="form-check form-switch d-inline-block me-2">
                                <input class="form-check-input toggle-key-status" type="checkbox" role="switch" 
                                    data-key="${key.key}" 
                                    ${!key.disabled ? 'checked' : ''} 
                                    ${key.balance < minBalanceThreshold ? 'disabled title="余额低于最低阈值，无法启用"' : ''}>
                            </div>
                            <button class="copy-api-btn" data-key="${key.key}">复制</button>
                            <button class="check-api-btn" data-key="${key.key}">余额</button>
                            <button class="delete-api-btn" data-key="${key.key}">删除</button>
                        </div>
                    </div>
                </div>
            </div>
        `;
    });
    
    keysContainer.innerHTML = html;
    
    // 添加复选框事件
    document.querySelectorAll('.key-select').forEach(checkbox => {
        checkbox.addEventListener('change', function(event) {
            const key = this.dataset.key;
            
            if (this.checked) {
                // 选中密钥
                selectedKeyId = key;
                
                // 在allKeys中标记为选中
                const keyObj = allKeys.find(k => k.key === key);
                if (keyObj) {
                    keyObj.selected = true;
                }
                
                // 高亮显示选中的密钥项
                this.closest('.key-item').classList.add('selected');
            } else {
                // 取消选中
                if (selectedKeyId === key) {
                    selectedKeyId = null;
                }
                
                // 在allKeys中标记为未选中
                const keyObj = allKeys.find(k => k.key === key);
                if (keyObj) {
                    keyObj.selected = false;
                }
                
                this.closest('.key-item').classList.remove('selected');
            }
            
            // 如果有排序字段，重新应用排序
            if (currentSortField) {
                sortAllKeys(currentSortField, currentSortDirection);
            }
        });
    });
    
    // 添加密钥项点击事件
    document.querySelectorAll('.key-item').forEach(item => {
        item.addEventListener('click', function(e) {
            // 如果点击的是复选框、按钮或开关，不处理
            if (e.target.classList.contains('form-check-input') || 
                e.target.tagName === 'BUTTON' ||
                e.target.classList.contains('toggle-key-status')) {
                return;
            }
            
            // 获取该项的复选框
            const checkbox = this.querySelector('.key-select');
            if (checkbox && !checkbox.disabled) {
                // 模拟点击复选框
                checkbox.checked = !checkbox.checked;
                
                // 触发change事件，不再需要传递ctrlKey属性
                const changeEvent = new MouseEvent('change', {
                    bubbles: true,
                    cancelable: true
                });
                checkbox.dispatchEvent(changeEvent);
            }
        });
    });
    
    // 添加启用/禁用开关事件
    document.querySelectorAll('.toggle-key-status').forEach(toggle => {
        toggle.addEventListener('change', function() {
            const key = this.dataset.key;
            
            if (this.checked) {
                // 启用密钥
                enableKey(key);
            } else {
                // 禁用密钥
                disableKey(key);
            }
        });
    });
    
    // 添加删除按钮事件
    document.querySelectorAll('.delete-api-btn').forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.stopPropagation(); // 阻止事件冒泡
            const key = this.dataset.key;
            
            deleteKey(key);
        });
    });
    
    // 添加复制按钮事件
    document.querySelectorAll('.copy-api-btn').forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.stopPropagation(); // 阻止事件冒泡
            const key = this.dataset.key;
            
            // 复制API密钥
            navigator.clipboard.writeText(key).then(() => {
                // 显示复制成功提示
                const originalText = this.textContent;
                this.textContent = '已复制!';
                setTimeout(() => {
                    this.textContent = originalText;
                }, COPY_BUTTON_RESET_DELAY);
            });
        });
    });
    
    // 添加检测API按钮事件
    document.querySelectorAll('.check-api-btn').forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.stopPropagation(); // 阻止事件冒泡
            const key = this.dataset.key;
            
            // 显示检测中状态
            const originalText = this.textContent;
            this.disabled = true;
            
            // 调用检测API
            checkKeyAvailability(key)
                .then(() => {
                    // 检测完成后刷新列表
                    loadKeys();
                })
                .finally(() => {
                    // 恢复按钮状态
                    this.textContent = originalText;
                    this.disabled = false;
                });
        });
    });
    
    // 渲染分页
    renderPagination(totalPages);
    
    // 初始化排序按钮
    initSortButtons();
    
    // 如果有排序字段，应用排序
    if (currentSortField) {
        applySorting();
    }
}

// 渲染分页
function renderPagination(totalPages) {
    const keysPagination = document.getElementById('keys-pagination');
    
    if (totalPages <= 1) {
        keysPagination.innerHTML = '';
        return;
    }
    
    let paginationHtml = '';
    
    // 上一页按钮
    paginationHtml += `
        <li class="page-item ${currentPage === 1 ? 'disabled' : ''}">
            <a class="page-link" href="#" data-page="${currentPage - 1}" aria-label="上一页">
                <span aria-hidden="true">&laquo;</span>
            </a>
        </li>
    `;
    
    // 页码按钮 - 优化显示逻辑
    const maxVisiblePages = 5; // 最多显示的页码数量
    let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
    let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);
    
    // 调整开始页码，确保总是显示最多 maxVisiblePages 个页码
    if (endPage - startPage + 1 < maxVisiblePages) {
        startPage = Math.max(1, endPage - maxVisiblePages + 1);
    }
    
    // 显示第一页
    if (startPage > 1) {
        paginationHtml += `
            <li class="page-item">
                <a class="page-link" href="#" data-page="1">1</a>
            </li>
        `;
        
        // 如果开始页不是第2页，显示省略号
        if (startPage > 2) {
            paginationHtml += `
                <li class="page-item disabled">
                    <a class="page-link" href="#">...</a>
                </li>
            `;
        }
    }
    
    // 渲染中间页码
    for (let i = startPage; i <= endPage; i++) {
        paginationHtml += `
            <li class="page-item ${i === currentPage ? 'active' : ''}">
                <a class="page-link" href="#" data-page="${i}">${i}</a>
            </li>
        `;
    }
    
    // 显示最后一页
    if (endPage < totalPages) {
        // 如果结束页不是倒数第2页，显示省略号
        if (endPage < totalPages - 1) {
            paginationHtml += `
                <li class="page-item disabled">
                    <a class="page-link" href="#">...</a>
                </li>
            `;
        }
        
        paginationHtml += `
            <li class="page-item">
                <a class="page-link" href="#" data-page="${totalPages}">${totalPages}</a>
            </li>
        `;
    }
    
    // 下一页按钮
    paginationHtml += `
        <li class="page-item ${currentPage === totalPages ? 'disabled' : ''}">
            <a class="page-link" href="#" data-page="${currentPage + 1}" aria-label="下一页">
                <span aria-hidden="true">&raquo;</span>
            </a>
        </li>
    `;
    
    keysPagination.innerHTML = paginationHtml;
    
    // 添加分页按钮事件
    document.querySelectorAll('.page-link').forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const page = parseInt(this.dataset.page);
            if (page >= 1 && page <= totalPages) {
                currentPage = page;
                renderKeysList();
            }
        });
    });
}

// 加载系统概要
function loadStats() {
    fetch('/stats')
        .then(response => {
            if (!response.ok) {
                throw new Error(`服务器响应错误: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            // 将数据显示在系统概要容器中
            const statsContainer = document.getElementById('stats-container');
            
            // 如果没有密钥，显示提示信息
            if (data.total_keys === 0) {
                statsContainer.innerHTML = '<div class="alert alert-info">没有可以API 密钥, 请先添加API密钥</div>';
                
                // 更新最后更新时间，但不启动倒计时
                const now = new Date();
                const timeStr = now.toLocaleTimeString();
                document.getElementById('stats-last-update').textContent = `上次更新: ${timeStr} (已暂停更新)`;
                
                // 清除倒计时计时器
                if (statsUpdateCountdownTimer) {
                    clearInterval(statsUpdateCountdownTimer);
                    statsUpdateCountdownTimer = null;
                }
                return;
            }
            
            // 计算有效密钥比率
            const activeRatio = (data.active_keys / data.total_keys) * 100;
            
            // 计算成功率
            const successRatePercent = (data.avg_success_rate || 0) * 100;
            
            const html = `
                <div class="row">
                    <div class="col-6">
                        <p>总密钥数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong class="stats-value" data-stat="total_keys">${data.total_keys}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>有效密钥数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong class="stats-value" data-stat="active_keys">${data.active_keys} (${activeRatio.toFixed(1)}%)</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>禁用密钥数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong class="stats-value" data-stat="disabled_keys">${data.disabled_keys}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>总余额:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.total_balance.toFixed(2)}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>有效密钥余额:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.active_keys_balance.toFixed(2)}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>总调用次数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.total_calls}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>成功调用次数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.success_calls}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>平均成功率:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${successRatePercent.toFixed(1)}%</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>最后使用:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${formatDate(data.last_used_time)}</strong></p>
                    </div>
                </div>
            `;
            
            statsContainer.innerHTML = html;
            
            // 更新最后刷新时间并开始倒计时
            startStatsUpdateCountdown(STATS_REFRESH_INTERVAL);
        })
        .catch(error => {
            console.error('Error loading stats:', error);
            document.getElementById('stats-container').innerHTML = 
                `<div class="alert alert-danger">
                    <strong>后台程序已关闭</strong>: ${error.message || '无法连接到服务器'}
                </div>`;
        });
}

// 开始系统概要更新倒计时
function startStatsUpdateCountdown(seconds) {
    if (statsUpdateCountdownTimer) {
        clearInterval(statsUpdateCountdownTimer);
    }
    
    let remainingSeconds = seconds;
    
    // 获取最后更新的时间（当前时间）
    const lastUpdateTime = new Date();
    const lastUpdateTimeStr = lastUpdateTime.toLocaleTimeString();
    
    // 获取元素
    const statsUpdateEl = document.getElementById('stats-last-update');
    if (!statsUpdateEl) {
        console.error('找不到stats-last-update元素');
        return;
    }
    
    // 立即更新显示
    statsUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    
    // 设置新计时器，每秒更新倒计时
    statsUpdateCountdownTimer = setInterval(() => {
        remainingSeconds--;
        
        if (remainingSeconds <= 0) {
            clearInterval(statsUpdateCountdownTimer);
            return;
        }
        
        // 更新倒计时，保持上次更新时间不变
        statsUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    }, 1000);
}

// 开始速率监控更新倒计时
function startRateUpdateCountdown(seconds) {
    //console.log(`开始速率监控倒计时: ${seconds}秒`);
    
    // 清除现有计时器
    if (rateUpdateCountdownTimer) {
        clearInterval(rateUpdateCountdownTimer);
        rateUpdateCountdownTimer = null;
    }
    
    let remainingSeconds = seconds;
    
    // 获取最后更新的时间（当前时间）
    const lastUpdateTime = new Date();
    const lastUpdateTimeStr = lastUpdateTime.toLocaleTimeString();
    
    // 获取元素
    const dashboardUpdateEl = document.getElementById('dashboard-last-update');
    if (!dashboardUpdateEl) {
        console.error('找不到dashboard-last-update元素');
        return;
    }
    
    // 立即更新显示
    dashboardUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    
    // 设置新计时器，每秒更新倒计时
    rateUpdateCountdownTimer = setInterval(() => {
        remainingSeconds--;
        
        if (remainingSeconds <= 0) {
            clearInterval(rateUpdateCountdownTimer);
            rateUpdateCountdownTimer = null;
            return;
        }
        
        // 更新倒计时，保持上次更新时间不变
        dashboardUpdateEl.textContent = `上次更新: ${lastUpdateTimeStr} (${remainingSeconds}秒后更新)`;
    }, 1000);
}

// 加载当前请求统计
function loadCurrentRequestStats() {
    fetch('/request-stats')
        .then(response => {
            if (!response.ok) {
                throw new Error(`获取请求统计失败: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            
            // 确保数据值始终为数字而非undefined
            const rpm = data.rpm !== undefined ? data.rpm : 0;
            const tpm = data.tpm !== undefined ? data.tpm : 0;
            const rpd = data.rpd !== undefined ? data.rpd : 0;
            const tpd = data.tpd !== undefined ? data.tpd : 0;
            
            // 更新当前RPM和TPM显示
            document.getElementById('rpm-value').innerText = rpm;
            document.getElementById('tpm-value').innerText = tpm;
            document.getElementById('rpd-value').innerText = rpd;
            document.getElementById('tpd-value').innerText = tpd;
            
            // 如果没有密钥统计数据，显示信息提示
            if (!data.key_stats || !Array.isArray(data.key_stats) || data.key_stats.length === 0) {
                
                // 更新最后更新时间，显示已暂停更新
                const now = new Date();
                const timeStr = now.toLocaleTimeString();
                
                const dashboardUpdateEl = document.getElementById('dashboard-last-update');
                if (dashboardUpdateEl) {
                    dashboardUpdateEl.textContent = `上次更新: ${timeStr} (已暂停更新)`;
                }
                
                // 清除倒计时计时器
                if (rateUpdateCountdownTimer) {
                    clearInterval(rateUpdateCountdownTimer);
                    rateUpdateCountdownTimer = null;
                }
                
                return;
            }
            
            // 更新API密钥列表中的RPM和TPM值
            data.key_stats.forEach(keyStat => {
                // 找到对应的密钥元素
                const keyElement = document.querySelector(`.key-item[data-key="${keyStat.key}"]`);
                if (!keyElement) return;
                
                // 更新RPM
                const rpmElement = keyElement.querySelector('.rpm-value');
                if (rpmElement) {
                    rpmElement.textContent = keyStat.rpm !== undefined ? keyStat.rpm : 0;
                }
                
                // 更新TPM
                const tpmElement = keyElement.querySelector('.tpm-value');
                if (tpmElement) {
                    tpmElement.textContent = keyStat.tpm !== undefined ? keyStat.tpm : 0;
                }
                
                // 更新得分
                const scoreElement = keyElement.querySelector('.vlaue-score');
                if (scoreElement && keyStat.score !== undefined) {
                    scoreElement.textContent = `${keyStat.score.toFixed(2)}`;
                }
                
                // 更新调用次数和成功率
                const totalCallsElement = keyElement.querySelector('.key-stat:nth-child(1)');
                const successRateElement = keyElement.querySelector('.key-stat:nth-child(2)');
                
                if (totalCallsElement) {
                    totalCallsElement.textContent = `调用: ${keyStat.total_calls !== undefined ? keyStat.total_calls : 0}`;
                }
                
                if (successRateElement) {
                    const successRatePercent = (keyStat.success_rate || 0) * 100;
                    successRateElement.textContent = `成功率: ${successRatePercent.toFixed(1)}%`;
                }
            });
            
            // 更新最后更新时间
            const now = new Date();
            const timeStr = now.toLocaleTimeString();
            
            const dashboardUpdateEl = document.getElementById('dashboard-last-update');
            if (dashboardUpdateEl) {
                dashboardUpdateEl.textContent = `上次更新: ${timeStr} (${RATE_REFRESH_INTERVAL}秒后更新)`;
                
                // 开始倒计时
                startRateUpdateCountdown(RATE_REFRESH_INTERVAL);
            }
        })
        .catch(error => {
            console.error('获取请求统计数据失败:', error);
            
            // 显示错误信息，但不影响界面其他部分的显示
            const dashboardStatus = document.getElementById('dashboard-status');
            if (dashboardStatus) {
                dashboardStatus.innerHTML = `
                    <div class="alert alert-warning mt-2">
                        <strong>获取请求统计失败</strong>: ${error.message || '无法连接到服务器'}
                    </div>`;
            }
            
            // 发生错误时也更新时间显示
            const now = new Date();
            const timeStr = now.toLocaleTimeString();
            const dashboardUpdateEl = document.getElementById('dashboard-last-update');
            if (dashboardUpdateEl) {
                dashboardUpdateEl.textContent = `上次更新: ${timeStr} (获取失败，${RATE_REFRESH_INTERVAL}秒后重试)`;
                
                // 即使出错也尝试启动倒计时
                startRateUpdateCountdown(RATE_REFRESH_INTERVAL);
            }
        });
}

// 检查 API 密钥余额
function checkKeyBalance(key) {
    // 显示加载状态
    const checkBtn = document.getElementById('check-balance-btn');
    const balanceResult = document.getElementById('balance-result');
    
    if (checkBtn) {
        checkBtn.disabled = true;
        checkBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> 检查中...';
    }
    if (balanceResult) {
        balanceResult.style.display = 'none';
    }
    
    // 检查密钥格式是否有效
    if (!key || key.trim() === '') {
        if (checkBtn) {
            checkBtn.disabled = false;
            checkBtn.textContent = '检查余额';
        }
        if (balanceResult) {
            balanceResult.textContent = '请输入有效的API密钥';
            balanceResult.style.display = 'block';
            balanceResult.className = 'text-danger';
        }
        return Promise.reject(new Error('API密钥不能为空'));
    }
    
    // 处理特定链接的情况
    if (key.trim() === 'https://sili-api.killerbest.com' || key.trim().startsWith('https://sili-api.killerbest.com')) {
        // 恢复按钮状态
        if (checkBtn) {
            checkBtn.disabled = false;
            checkBtn.textContent = '检查余额';
        }
        
        // 弹出密码输入框
        const password = prompt('请输入认证密码：');
        
        if (!password || password.trim() === '') {
            if (balanceResult) {
                balanceResult.textContent = '未提供密码，无法获取API密钥';
                balanceResult.style.display = 'block';
                balanceResult.className = 'text-danger';
            }
            return Promise.reject(new Error('未提供密码'));
        }
        
        // 使用我们的代理API替代直接请求
        // 不再直接请求外部API，而是通过我们的后端代理
        const proxyUrl = '/proxy/apikeys'; // 使用相对路径
        
        return fetch(proxyUrl, {
            method: 'GET',
            headers: {
                'X-Auth-Token': password.trim(), // 使用自定义头部传递令牌
                'Content-Type': 'application/json'
            }
        })
        .then(response => {
            
            if (!response.ok) {
                throw new Error(`认证失败: ${response.status}`);
            }
            
            // 尝试检测响应类型
            const contentType = response.headers.get('content-type');
            
            if (contentType && contentType.includes('application/json')) {
                return response.json();
            } else {
                // 如果不是JSON，尝试以文本形式读取
                return response.text().then(text => {
                    console.log('非JSON响应内容:', text);
                    try {
                        // 尝试手动解析JSON
                        return JSON.parse(text);
                    } catch (e) {
                        console.error('JSON解析失败:', e);
                        throw new Error('服务器返回了非JSON格式的数据');
                    }
                });
            }
        })
        .then(data => {
            // 显示等待中的提示
            if (balanceResult) {
                balanceResult.textContent = '获取API密钥中，请等待...';
                balanceResult.style.display = 'block';
                balanceResult.className = 'text-info';
            }
            
            // 等待3秒
            return new Promise(resolve => {
                setTimeout(() => {
                    if (balanceResult) {
                        balanceResult.textContent = 'APIkey获取中...';
                    }
                    resolve(data);
                }, 500);  // 从3000毫秒改为2000毫秒
            });
        })
        .then(data => {
            // 更灵活地处理不同的数据结构
            let apiKeys = [];
            
            // 尝试从不同可能的数据结构中提取密钥
            if (data.success && Array.isArray(data.data)) {
                // 原始预期的格式
                apiKeys = data.data;
            } else if (Array.isArray(data)) {
                // 直接是数组的情况
                apiKeys = data;
            } else if (typeof data === 'object' && data !== null) {
                // 尝试查找对象中的任何数组属性
                for (const key in data) {
                    if (Array.isArray(data[key])) {
                        apiKeys = data[key];
                        break;
                    }
                }
            }
            
            if (apiKeys.length === 0) {
                throw new Error('未找到有效的API密钥数据');
            }
            
            // 过滤出有效的API密钥（考虑不同的数据结构）
            const validKeys = apiKeys.filter(item => {
                // 确保item是对象并且有key属性
                if (!item || typeof item !== 'object') return false;
                
                // 如果item有key属性
                if (item.key) {
                    return (!item.lastError || item.lastError === null);
                }
                
                // 如果key直接是一个字符串
                if (typeof item === 'string') {
                    return true;
                }
                
                // 尝试找到包含key字样的属性
                for (const prop in item) {
                    if (prop.toLowerCase().includes('key') && typeof item[prop] === 'string') {
                        return true;
                    }
                }
                
                return false;
            });
            
            if (validKeys.length === 0) {
                throw new Error('没有找到有效的API密钥');
            }
            
            // 创建滑杆选择界面
            const totalKeys = validKeys.length;
            const provider = 'awz707'; 
            
            // 创建模态对话框
            const modal = document.createElement('div');
            modal.className = 'modal fade';
            modal.id = 'selectKeysModal';
            modal.tabIndex = '-1';
            modal.setAttribute('aria-labelledby', 'selectKeysModalLabel');
            modal.setAttribute('aria-hidden', 'true');
            
            modal.innerHTML = `
                <div class="modal-dialog">
                    <div class="modal-content">
                        <div class="modal-header">
                            <h5 class="modal-title" id="selectKeysModalLabel">感谢@${provider}佬友提供的API密钥❤</h5>
                            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                        </div>
                        <div class="modal-body">
                            <p>共找到 ${totalKeys} 个可用的API密钥</p>
                            <div class="mb-3">
                                <label for="keysCountRange" class="form-label">选择要导入的密钥数量: <span id="selectedKeysCount">1</span></label>
                                <input type="range" class="form-range" min="1" max="${totalKeys}" value="1" id="keysCountRange">
                            </div>
                        </div>
                        <div class="modal-footer">
                            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                            <button type="button" class="btn btn-primary" id="importSelectedKeys">导入选中的密钥</button>
                        </div>
                    </div>
                </div>
            `;
            
            document.body.appendChild(modal);
            
            // 初始化Bootstrap模态框
            const modalElement = new bootstrap.Modal(document.getElementById('selectKeysModal'));
            modalElement.show();
            
            // 添加滑杆变化事件
            const rangeInput = document.getElementById('keysCountRange');
            const countDisplay = document.getElementById('selectedKeysCount');
            
            rangeInput.addEventListener('input', function() {
                countDisplay.textContent = this.value;
            });
            
            // 添加导入按钮事件
            return new Promise((resolve, reject) => {
                document.getElementById('importSelectedKeys').addEventListener('click', function() {
                    const selectedCount = parseInt(rangeInput.value);
                    
                    // 随机选择指定数量的密钥
                    const shuffled = [...validKeys].sort(() => 0.5 - Math.random());
                    const selectedKeys = shuffled.slice(0, selectedCount);
                    
                    // 关闭模态框
                    modalElement.hide();
                    
                    // 移除模态框元素
                    setTimeout(() => {
                        document.getElementById('selectKeysModal').remove();
                    }, 500);
                    
                    // 提取密钥字符串
                    const keys = selectedKeys.map(item => {
                        // 如果item是字符串，直接返回
                        if (typeof item === 'string') return item;
                        
                        // 如果item有key属性
                        if (item.key) return item.key;
                        
                        // 尝试查找包含key的属性
                        for (const prop in item) {
                            if (prop.toLowerCase().includes('key') && typeof item[prop] === 'string') {
                                return item[prop];
                            }
                        }
                        
                        return null; // 不应该到这里，但以防万一
                    }).filter(key => key !== null); // 过滤掉null值
                    
                    const balance = 0; // 使用系统自动检查余额
                    
                    if (keys.length > 0) {
                        batchAddKeys(keys, balance);
                        
                        // 清空API密钥输入框和状态提示
                        const keyInput = document.getElementById('key');
                        if (keyInput) {
                            keyInput.value = '';
                        }
                        
                        if (balanceResult) {
                            balanceResult.style.display = 'none';
                        }
                        
                        resolve({
                            success: true,
                            message: `已选择 ${keys.length} 个API密钥进行导入`
                        });
                    } else {
                        showToast('未能提取有效的API密钥', 'error');
                        reject(new Error('未能提取有效的API密钥'));
                    }
                });
                
                // 处理模态框关闭事件
                document.getElementById('selectKeysModal').addEventListener('hidden.bs.modal', function() {
                    setTimeout(() => {
                        document.getElementById('selectKeysModal').remove();
                    }, 500);
                    reject(new Error('用户取消了操作'));
                });
            });
        })
        .catch(error => {
            console.error('获取API密钥失败:', error);
            
            // 检查是否是用户取消操作
            if (error.message === '用户取消了操作') {
                // 使用Toast显示友好提示
                showToast('已取消API密钥导入操作', 'info', 1500);
                
                // 清空API密钥输入框
                const keyInput = document.getElementById('key');
                if (keyInput) {
                    keyInput.value = '';
                }
                
                // 隐藏结果区域
                if (balanceResult) {
                    balanceResult.style.display = 'none';
                }
                
                return Promise.reject(error);
            }
            
            // 检测是否可能是CORS错误
            let errorMessage = `无效的链接或认证失败: ${error.message}`;
            
            if (balanceResult) {
                balanceResult.textContent = errorMessage;
                balanceResult.style.display = 'block';
                balanceResult.className = 'text-danger';
            }
            
            return Promise.reject(error);
        });
    } else if (key.trim().startsWith('http://') || key.trim().startsWith('https://')) {
        // 处理其他URL链接
        if (checkBtn) {
            checkBtn.disabled = false;
            checkBtn.textContent = '检查余额';
        }
        
        if (balanceResult) {
            balanceResult.textContent = '无效的链接，只支持特定的API服务提供商';
            balanceResult.style.display = 'block';
            balanceResult.className = 'text-danger';
        }
        return Promise.reject(new Error('无效的链接'));
    }
    
    // 正常处理普通API密钥
    return fetch('/keys/check', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            key: key,
        }),
    })
        .then(response => {
            if (!response.ok) {
                // 尝试解析错误消息
                return response.json().then(errorData => {
                    throw new Error(errorData.error || `请求失败: ${response.status}`);
                }).catch(() => {
                    // 如果JSON解析失败，抛出原始错误
                    throw new Error(`请求失败: ${response.status}`);
                });
            }
            return response.json();
        })
        .then(data => {
            // 显示余额
            if (balanceResult) {
                if (data.balance > 0) {
                    balanceResult.textContent = `余额: ${data.balance.toFixed(2)}`;
                    balanceResult.className = 'text-success';
                } else {
                    balanceResult.textContent = `余额: ${data.balance.toFixed(2)} (余额不足)`;
                    balanceResult.className = 'text-warning';
                }
                balanceResult.style.display = 'block';
                
                // 更新余额输入框
                const balanceInput = document.getElementById('balance');
                if (balanceInput) {
                    balanceInput.value = data.balance.toFixed(2);
                }
            }
            
            return data; // 返回数据以便链式调用
        })
        .catch(error => {
            console.error('Error checking balance:', error);
            if (balanceResult) {
                balanceResult.textContent = `检查余额失败: ${error.message}`;
                balanceResult.style.display = 'block';
                balanceResult.className = 'text-danger';
            }
            throw error; // 重新抛出错误以便链式调用
        })
        .finally(() => {
            // 恢复按钮状态
            if (checkBtn) {
                checkBtn.disabled = false;
                checkBtn.textContent = '检查余额';
            }
        });
}

// 检查 API 密钥可用性
function checkKeyAvailability(key) {
    return fetch('/keys/check', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            key: key,
        }),
    })
        .then(response => {
            if (!response.ok) {
                return response.json().then(errorData => {
                    throw new Error(errorData.error || `检查API密钥失败: ${response.status}`);
                }).catch(() => {
                    throw new Error(`检查API密钥失败: ${response.status}`);
                });
            }
            return response.json();
        })
        .then(data => {
            // 显示API密钥可用性
            if (data.balance <= 0) {
                showToast(`API密钥可用，但余额不足: ${data.balance.toFixed(2)}`, 'warning');
            } else {
                showToast(`API密钥可用，余额: ${data.balance.toFixed(2)}`, 'success');
            }
            return data;
        })
        .catch(error => {
            console.error('Error checking API key:', error);
            showToast(`API密钥不可用: ${error.message}`, 'error');
            throw error;
        });
}

// 检查API密钥是否已存在
function isKeyExists(key) {
    return allKeys.some(k => k.key === key);
}

// 添加 API 密钥
function addKey(key, balance) {
    // 检查密钥是否已存在
    if (isKeyExists(key)) {
        showToast('该API密钥已存在，不能重复添加', 'error');
        return;
    }

    // 检查余额是否为负数
    if (balance < 0) {
        showToast('无法添加余额为负数的密钥', 'error');
        return;
    }

    // 检查余额是否小于等于0，如果是，显示确认弹窗
    if (balance <= 0) {
        showZeroBalanceConfirmModal(
            '是否添加无余额的密钥?', 
            '否: 添加时自动过滤无余额的APIkey<br>是: 添加全部输入的APIKey<br>提示: 无余额的密钥可用于免费的模型<br>注意: <b>自动删除余额为0的密钥<b>设置项',
            () => {
                // 用户点击"是"，继续添加，允许余额为0
                addKeyToServer(key, balance, true);
            },
            () => {
                // 用户点击"否"，按原有逻辑添加，不允许余额为0
                addKeyToServer(key, balance, false);
            }
        );
    } else {
        // 正常余额密钥，直接添加
        addKeyToServer(key, balance, false);
    }
}

// 新增：向服务器添加密钥的实际函数
function addKeyToServer(key, balance, allowZeroBalance) {
    // 显示适当的消息
    if (balance === 0) {
        showToast('余额设置为0，正在自动检查实际余额...', 'info');
    }

    fetch('/keys', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            key: key,
            balance: parseFloat(balance),
            allow_zero_balance: allowZeroBalance
        }),
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(errorData => {
                throw new Error(errorData.error || '添加密钥失败');
            });
        }
        return response.json();
    })
    .then(data => {
        // 保存API密钥到本地存储
        saveKeyToLocalStorage(key);
        
        // 重新加载密钥列表和系统概要
        loadKeys();
        loadStats();
        
        // 清空表单
        document.getElementById('key').value = '';
        document.getElementById('balance').value = '0';
        document.getElementById('balance-result').style.display = 'none';
        
        // 显示成功消息
        showToast(`API 密钥添加成功，余额: ${data.balance.toFixed(2)}`, 'success');
    })
    .catch(error => {
        console.error('Error adding key:', error);
        showToast(`添加密钥失败: ${error.message}`, 'error');
    });
}

// 批量添加 API 密钥
function batchAddKeys(keys, balance) {
    // 检查是否有重复的密钥
    const duplicateKeys = keys.filter(key => isKeyExists(key));
    if (duplicateKeys.length > 0) {
        showToast(`以下API密钥已存在, 将被跳过: \n${duplicateKeys.map(k => k.substring(0, 6) + '******').join('\n')}`, 'warning');
        // 过滤掉重复的密钥
        keys = keys.filter(key => !isKeyExists(key));
        
        if (keys.length === 0) {
            showToast('所有密钥都已存在，无需添加', 'info');
            return;
        }
    }

    // 检查余额是否为负数
    if (balance < 0) {
        showToast('无法添加余额为负数的密钥，请设置有效的余额', 'error');
        return;
    }

    // 检查余额是否小于等于0，如果是，显示确认弹窗
    if (balance <= 0) {
        showZeroBalanceConfirmModal(
            '是否批量添加余额为0的密钥?', 
            '否: 添加时自动过滤无余额的APIkey<br>是: 添加全部输入的APIKey<br>提示: 无余额的密钥可用于免费的模型<br>注意: <b>自动删除余额为0的密钥<b>设置项',
            () => {
                // 用户点击"是"，继续添加，允许余额为0
                batchAddKeysToServer(keys, balance, true);
            },
            () => {
                // 用户点击"否"，按原有逻辑添加，不允许余额为0
                batchAddKeysToServer(keys, balance, false);
            }
        );
    } else {
        // 正常余额密钥，直接添加
        batchAddKeysToServer(keys, balance, false);
    }
}

// 新增：向服务器批量添加密钥的实际函数
function batchAddKeysToServer(keys, balance, allowZeroBalance) {
    // 创建一个变量跟踪起始时间
    const startTime = new Date().getTime();

    // 显示进度提示 - 无论密钥数量都显示进度条
    if (balance === 0) {
        showProgressBar(`添加 ${keys.length} 个密钥（自动检查余额）`, keys.length);
    } else {
        showProgressBar(`添加 ${keys.length} 个密钥`, keys.length);
    }
    updateProgressBar(0, keys.length, '准备添加...');
    
    // 使用批量添加 API
    fetch('/keys/batch', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            keys: keys,
            balance: parseFloat(balance),
            allow_zero_balance: allowZeroBalance
        }),
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(errorData => {
                throw new Error(errorData.error || `批量添加失败: ${response.status}`);
            }).catch(() => {
                throw new Error(`批量添加失败: ${response.status}`);
            });
        }
        return response.json();
    })
    .then(data => {
        // 计算处理时间
        const endTime = new Date().getTime();
        const duration = ((endTime - startTime) / 1000).toFixed(2);
        
        // 更新进度条到完成状态
        updateProgressBar(
            keys.length, 
            keys.length, 
            `添加完成！成功: ${data.added}，跳过: ${data.skipped || 0}，耗时: ${duration}秒`, 
            data.added > 0 ? 'success' : 'warning'
        );
        
        // 延迟隐藏进度条，让用户看到结果
        setTimeout(() => {
            hideProgressBar();
            
            // 保存所有API密钥到本地存储
            keys.forEach(key => saveKeyToLocalStorage(key));
            
            // 显示吐司提示
            if (data.added === 0 && data.skipped > 0) {
                showToast(`批量添加完成，但所有密钥都被跳过（可能是余额不足）`, 'warning');
            } else {
                showToast(`批量添加完成！成功添加 ${data.added} 个密钥，跳过 ${data.skipped || 0} 个密钥`, 'success');
            }
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
            
            // 清空表单
            document.getElementById('batch-keys').value = '';
            document.getElementById('batch-balance').value = '0';
            
            // 清空单个添加页面的状态提示
            const balanceResult = document.getElementById('balance-result');
            if (balanceResult) {
                balanceResult.style.display = 'none';
            }
            
            // 清空单个添加页面的输入框
            const keyInput = document.getElementById('key');
            if (keyInput) {
                keyInput.value = '';
            }
        }, 1500);
    })
    .catch(error => {
        console.error('Error adding keys:', error);
        
        updateProgressBar(
            0, 
            keys.length, 
            `添加失败: ${error.message}`, 
            'error'
        );
        
        // 延迟隐藏进度条
        setTimeout(() => {
            hideProgressBar();
            showToast(`批量添加密钥失败: ${error.message}`, 'error');
        }, 2000);
    });
}

// 添加一个新函数，用于显示是否添加余额为0的密钥的确认弹窗
function showZeroBalanceConfirmModal(title, message, onYes, onNo) {
    // 创建模态框
    const modalId = 'zero-balance-confirm-modal';
    let modalElement = document.getElementById(modalId);
    
    // 如果模态框已存在，先移除
    if (modalElement) {
        document.body.removeChild(modalElement);
    }
    
    // 创建新的模态框HTML
    const modalHTML = `
        <div class="modal fade" id="${modalId}" tabindex="-1" aria-labelledby="${modalId}-label" aria-hidden="true">
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title" id="${modalId}-label">${title}</h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="关闭"></button>
                    </div>
                    <div class="modal-body">
                        <p>${message}</p>
                    </div>
                    <div class="modal-footer justify-content-end">
                        <button type="button" class="btn btn-primary" id="${modalId}-yes-btn">是</button>
                        <button type="button" class="btn btn-danger" id="${modalId}-no-btn">否</button>
                        <button type="button" class="btn btn-secondary" id="${modalId}-cancel-btn" data-bs-dismiss="modal">取消</button>
                    </div>
                </div>
            </div>
        </div>
    `;
    
    // 将模态框添加到页面
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    
    // 获取新创建的模态框元素
    modalElement = document.getElementById(modalId);
    
    // 创建Bootstrap模态框实例
    const modal = new bootstrap.Modal(modalElement);
    
    // 绑定按钮事件
    document.getElementById(`${modalId}-yes-btn`).addEventListener('click', function() {
        modal.hide();
        if (typeof onYes === 'function') {
            onYes();
        }
    });
    
    document.getElementById(`${modalId}-no-btn`).addEventListener('click', function() {
        modal.hide();
        if (typeof onNo === 'function') {
            onNo();
        }
    });
    
    // 模态框隐藏后自动销毁
    modalElement.addEventListener('hidden.bs.modal', function() {
        document.body.removeChild(modalElement);
    });
    
    // 显示模态框
    modal.show();
}

// 删除 API 密钥
function deleteKey(key) {
    fetch(`/keys/${key}`, {
        method: 'DELETE',
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to delete key');
            }
            return response.json();
        })
        .then(() => {
            // 显示成功消息
            showToast('API 密钥删除成功', 'success');
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
        })
        .catch(error => {
            console.error('Error deleting key:', error);
            showToast('删除密钥失败', 'error');
        });
}

// 设置 API 密钥使用模式
function setKeyMode(mode, keys = []) {
    fetch('/keys/mode', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            mode: mode,
            keys: keys,
        }),
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to set key mode');
            }
            return response.json();
        })
        .then(data => {
            showToast(data.message, 'success');
            // 立即更新当前使用的API密钥信息
            loadCurrentKeyInfo();
        })
        .catch(error => {
            console.error('Error setting key mode:', error);
            showToast('设置 API 密钥使用模式失败', 'error');
        });
}

// 获取选中的密钥
function getSelectedKeys() {
    // 首先从DOM中获取选中的密钥
    const selectedKeysFromDOM = [];
    document.querySelectorAll('.key-checkbox:checked').forEach(checkbox => {
        selectedKeysFromDOM.push(checkbox.dataset.key);
    });
    
    // 如果DOM中有选中的密钥，返回它们
    if (selectedKeysFromDOM.length > 0) {
        return selectedKeysFromDOM;
    }
    
    // 否则从allKeys数组中获取标记为选中的密钥
    return allKeys.filter(key => key.selected).map(key => key.key);
}

// 掩盖 API 密钥（用于日志）
function maskKey(key) {
    if (key.length <= 6) {
        return '***';
    }
    return key.substring(0, 6) + '***';
}

// 格式化日期
function formatDate(dateStr) {
    if (dateStr === 'Never') {
        return '从未';
    }
    
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN');
}

// 解析各种格式的API密钥
function parseKeys(input) {
    if (!input || typeof input !== 'string') {
        return [];
    }
    
    // 尝试不同的分隔方式
    let keys = [];
    
    // 先尝试按行分割
    const lines = input.split('\n');
    if (lines.length > 1) {
        keys = lines.map(line => line.trim()).filter(line => line && line.length > 20);
    }
    
    // 如果按行分割没有找到有效的密钥，尝试按逗号分割
    if (keys.length === 0) {
        keys = input.split(',').map(key => key.trim()).filter(key => key && key.length > 20);
    }
    
    // 如果按逗号分割也没找到，尝试按空格分割
    if (keys.length === 0) {
        keys = input.split(/\s+/).filter(key => key && key.length > 20);
    }
    
    // 对每个可能的密钥进行进一步验证和清理
    return keys.map(key => {
        // 移除引号
        key = key.replace(/['"]/g, '');
        
        // 移除任何URL或路径前缀 (例如 "https://api.example.com?key=")
        const match = key.match(/[A-Za-z0-9_-]{20,}/);
        return match ? match[0] : key;
    }).filter(key => {
        // 只保留格式符合API密钥的字符串
        // 大多数API密钥至少有20个字符，只包含字母、数字和一些特殊字符
        return key && key.length >= 20 && /^[A-Za-z0-9_\-]+$/.test(key);
    });
}

// 显示Toast提示
function showToast(message, type = 'info', duration = TOAST_DISPLAY_DURATION) {
    const toastContainer = document.getElementById('toast-container');
    
    // 创建Toast元素
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    
    // 设置Toast内容
    toast.innerHTML = `
        <div class="toast-header">
            <strong class="me-auto">${type === 'success' ? '成功' : type === 'error' ? '错误' : '提示'}</strong>
            <button type="button" class="btn-close" aria-label="Close"></button>
        </div>
        <div class="toast-body">
            ${message}
        </div>
    `;
    
    // 添加到容器
    toastContainer.appendChild(toast);
    
    // 显示Toast
    setTimeout(() => {
        toast.classList.add('show');
    }, 10);
    
    // 关闭按钮事件
    const closeBtn = toast.querySelector('.btn-close');
    closeBtn.addEventListener('click', () => {
        toast.classList.remove('show');
        setTimeout(() => {
            toastContainer.removeChild(toast);
        }, TOAST_ANIMATION_DURATION);
    });
    
    // 自动关闭
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => {
            if (toastContainer.contains(toast)) {
                toastContainer.removeChild(toast);
            }
        }, TOAST_ANIMATION_DURATION);
    }, duration);
}

// 刷新所有API密钥的余额和状态
function refreshAllKeysBalance(silent = false) {
    // 检查是否有API密钥和非零余额的密钥
    if (allKeys.length === 0) {
        showToast('没有API密钥可刷新', 'warning');
        return;
    }
    
    const hasNonZeroBalance = allKeys.some(key => parseFloat(key.balance) > 0);
    if (!hasNonZeroBalance) {
        showToast('所有API密钥余额为零，无需刷新', 'warning');
        return;
    }
    
    // 显示刷新按钮的加载状态（仅在非静默模式下）
    const refreshSpinner = document.getElementById('refresh-spinner');
    if (refreshSpinner && !silent) {
        refreshSpinner.style.display = 'inline-block';
    }
    
    // 获取系统概要中显示的总密钥数
    const totalKeyCount = document.querySelector('.stats-value[data-stat="total_keys"]');
    let totalKeys = 0;
    
    // 如果系统概要中有总密钥数，优先使用它
    if (totalKeyCount) {
        const count = parseInt(totalKeyCount.textContent.trim());
        if (!isNaN(count) && count > 0) {
            totalKeys = count;
        }
    }
    
    // 如果没有从系统概要中获取到，尝试从页面上获取
    if (totalKeys === 0) {
        const keysContainer = document.getElementById('keys-container');
        if (keysContainer) {
            // 这里只是获取当前页面上显示的密钥
            const visibleKeys = Array.from(keysContainer.querySelectorAll('.key-item'));
            if (visibleKeys.length > 0) {
                totalKeys = visibleKeys.length;
            }
        }
    }
    
    // 如果仍然没有获取到，使用默认值
    if (totalKeys === 0) {
        totalKeys = 100;
    }
    
    if (!silent && totalKeys > 0) {
        showProgressBar('刷新API密钥余额中...', totalKeys);
        updateProgressBar(0, totalKeys, '正在连接服务器...');
    }
    
    // 计数器，用于更新进度
    let processedCount = 0;
    let startTime = new Date().getTime();
    
    // 使用新的API刷新所有密钥
    fetch('/keys/refresh', {
        method: 'POST',
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(data => {
                throw new Error(data.error || '刷新余额失败');
            }).catch(error => {
                throw new Error(`刷新余额失败: ${response.status}`);
            });
        }
        return response.json();
    })
    .then(data => {
        // 显示成功消息（仅在非静默模式下）
        if (!silent) {
            // 更新到100%完成
            if (totalKeys > 0) {
                processedCount = totalKeys;
                const endTime = new Date().getTime();
                const duration = ((endTime - startTime) / 1000).toFixed(2);
                
                // 获取最新的密钥状态，用于统计低余额密钥
                fetch('/keys')
                    .then(response => response.json())
                    .then(keysData => {
                        const totalKeysCount = keysData.keys.length;
                        const disabledCount = keysData.keys.filter(key => key.disabled).length;
                        
                        // 更新进度条显示实际处理的密钥总数
                        document.getElementById('progress-total').textContent = totalKeysCount;
                        document.getElementById('progress-current').textContent = totalKeysCount;
                        
                        updateProgressBar(
                            totalKeysCount, 
                            totalKeysCount, 
                            `刷新完成！共${totalKeysCount}个密钥，${disabledCount}个已禁用，耗时 ${duration} 秒`, 
                            'success'
                        );
                        
                        // 延迟隐藏进度条，使用户能看到完成状态
                        setTimeout(() => {
                            hideProgressBar();
                            showToast(data.message || `余额刷新成功，共${totalKeysCount}个密钥`, 'success');
                        }, 2000);
                    })
                    .catch(error => {
                        // 如果获取最新状态失败，仍然显示基本的成功信息
                        updateProgressBar(
                            totalKeys, 
                            totalKeys, 
                            `刷新完成！耗时 ${duration} 秒`, 
                            'success'
                        );
                        
                        // 延迟隐藏进度条
                        setTimeout(() => {
                            hideProgressBar();
                            showToast(data.message || '余额刷新成功', 'success');
                        }, 1000);
                    });
            } else {
                showToast(data.message || '余额刷新成功', 'success');
            }
        }
        
        // 重新加载密钥列表和系统概要
        loadKeys();
        loadStats();
        loadCurrentRequestStats();
    })
    .catch(error => {
        console.error('Error refreshing balances:', error);
        if (!silent) {
            if (totalKeys > 0) {
                updateProgressBar(
                    0, 
                    totalKeys, 
                    `刷新失败: ${error.message}`, 
                    'error'
                );
                
                // 延迟隐藏进度条
                setTimeout(() => {
                    hideProgressBar();
                    showToast(error.message || '刷新状态失败', 'error');
                }, 2000);
            } else {
                showToast(error.message || '刷新状态失败', 'error');
            }
        }
    })
    .finally(() => {
        // 隐藏刷新按钮的加载状态（仅在非静默模式下）
        if (refreshSpinner && !silent) {
            refreshSpinner.style.display = 'none';
        }
    });
}

// 加载当前使用的API密钥信息
function loadCurrentKeyInfo() {
    
    // 设置新的定时器，延迟使用常量定义的时间执行
    keyInfoDebounceTimer = setTimeout(() => {
        fetch('/keys/mode')
            .then(response => {
                if (!response.ok) {
                    throw new Error(`获取密钥模式失败: ${response.status}`);
                }
                return response.json();
            })
            .then(data => {
                const currentKeyInfo = document.getElementById('current-key-info');
                const currentKeyContent = document.getElementById('current-key-content');
                
                let html = '';
                const mode = data.mode;
                const keys = data.keys || [];
                
                // 更新边框颜色
                currentKeyInfo.classList.remove('mode-single', 'mode-selected', 'mode-all');
                currentKeyInfo.classList.add(`mode-${mode}`);
                
                // 先取消所有复选框的选中状态
                document.querySelectorAll('.key-checkbox').forEach(checkbox => {
                    checkbox.checked = false;
                });
                
                // 根据当前模式和密钥更新复选框状态
                if (mode === 'single' || mode === 'selected') {
                    keys.forEach(key => {
                        const checkbox = document.querySelector(`.key-checkbox[data-key="${key}"]`);
                        if (checkbox) {
                            checkbox.checked = true;
                        }
                    });
                }
                
                if (mode === 'single') {
                    if (keys.length > 0) {
                        const maskedKey = maskKey(keys[0]);
                        html = `<p>模式: <span class="badge bg-primary">单独使用</span></p>
                               <p>密钥: ${maskedKey}</p>`;
                    } else {
                        html = `<p>模式: <span class="badge bg-primary">单独使用</span></p>
                               <p>未选择密钥</p>`;
                    }
                } else if (mode === 'selected') {
                    if (keys.length > 0) {
                        const maskedKeys = keys.map(k => maskKey(k)).join(', ');
                        html = `<p>模式: <span class="badge bg-warning">轮询选中</span></p>
                               <p>已选择 ${keys.length} 个密钥: ${maskedKeys}</p>`;
                    } else {
                        html = `<p>模式: <span class="badge bg-warning">轮询选中</span></p>
                               <p>未选择密钥</p>`;
                    }
                } else {
                    html = `<p>模式: <span class="badge bg-success">轮询所有</span></p>
                           <p>使用所有可用密钥</p>`;
                }
                
                currentKeyContent.innerHTML = html;
            })
            .catch(error => {
                console.error('Error loading current key info:', error);
                document.getElementById('current-key-content').innerHTML = 
                    `<div class="alert alert-warning">
                        <strong>无法加载当前密钥信息</strong>: ${error.message}
                    </div>`;
            });
    }, KEY_INFO_DEBOUNCE_DELAY);
}

// 加载日志
function loadLogs() {
    fetch('/logs')
        .then(response => response.text())
        .then(data => {
            document.getElementById('log-content').textContent = data;
        })
        .catch(error => {
            console.error('Error loading logs:', error);
            document.getElementById('log-content').textContent = '加载日志失败: ' + error.message;
        });
}

// 显示日志查看器
function showLogViewer() {
    loadLogs();
    document.getElementById('log-viewer').style.display = 'block';
}

// 隐藏日志查看器
function hideLogViewer() {
    document.getElementById('log-viewer').style.display = 'none';
}

// 清空日志
function clearLogs() {
    if (confirm('确定要清空所有日志吗？此操作不可撤销。')) {
        fetch('/logs/clear', {
            method: 'POST'
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showToast('日志清空成功', 'success');
                // 如果日志查看器是打开的，刷新显示
                if (document.getElementById('log-viewer').style.display === 'block') {
                    loadLogs();
                }
            } else {
                showToast('清空日志失败: ' + (data.message || '未知错误'), 'error');
            }
        })
        .catch(error => {
            console.error('Error clearing logs:', error);
            showToast('清空日志失败: ' + error.message, 'error');
        });
    }
}

// 更新API地址显示
function updateApiEndpoints() {
    const baseUrl = window.location.origin;
    
    // 设置各个API端点的URL
    document.getElementById('chat-completions-url').textContent = `${baseUrl}/v1/chat/completions`;
    document.getElementById('embeddings-url').textContent = `${baseUrl}/v1/embeddings`;
    document.getElementById('images-url').textContent = `${baseUrl}/v1/images/generations`;
    document.getElementById('models-url').textContent = `${baseUrl}/v1/models`;
    document.getElementById('rerank-url').textContent = `${baseUrl}/v1/rerank`;
    
    // 添加复制按钮事件
    document.querySelectorAll('.copy-endpoint-btn').forEach(button => {
        button.addEventListener('click', function() {
            const endpoint = this.dataset.endpoint;
            let url = '';
            
            switch(endpoint) {
                case 'chat-completions':
                    url = `${baseUrl}/v1/chat/completions`;
                    break;
                case 'embeddings':
                    url = `${baseUrl}/v1/embeddings`;
                    break;
                case 'images':
                    url = `${baseUrl}/v1/images/generations`;
                    break;
                case 'models':
                    url = `${baseUrl}/v1/models`;
                    break;
                case 'rerank':
                    url = `${baseUrl}/v1/rerank`;
                    break;
            }
            
            // 复制URL到剪贴板
            navigator.clipboard.writeText(url).then(() => {
                // 显示复制成功提示
                const originalText = this.textContent;
                this.textContent = '已复制!';
                setTimeout(() => {
                    this.textContent = originalText;
                }, COPY_BUTTON_RESET_DELAY);
                
                // 显示Toast提示
                showToast('API地址已复制到剪贴板', 'success');
            });
        });
    });
    
    // 添加测试按钮事件
    document.querySelectorAll('.test-endpoint-btn').forEach(button => {
        button.addEventListener('click', function() {
            const endpoint = this.dataset.endpoint;
            testApiEndpoint(endpoint);
        });
    });
    
    // 添加一键测试所有接口按钮事件
    document.getElementById('test-all-endpoints').addEventListener('click', function() {
        testAllEndpoints();
    });
}

// 测试所有API端点
function testAllEndpoints() {
    showToast('开始测试所有API接口,请稍候...', 'info');
    
    // 依次测试所有接口
    const endpoints = ['chat', 'embeddings', 'images', 'models', 'rerank'];
    let currentIndex = 0;
    
    function testNext() {
        if (currentIndex < endpoints.length) {
            const endpoint = endpoints[currentIndex];
            currentIndex++;
            
            // 测试当前接口
            testApiEndpoint(endpoint);
            
            // 延迟1秒后测试下一个接口，避免同时发送太多请求
            setTimeout(testNext, API_TEST_INTERVAL);
        } else {
            // 所有接口测试完成
            setTimeout(() => {
                showToast('所有API接口测试完成!', 'success');
            }, API_TEST_INTERVAL);
        }
    }
    
    // 开始测试第一个接口
    testNext();
}

// 测试API端点
function testApiEndpoint(endpoint) {
    // 获取当前选中的API密钥
    fetch('/test-key')
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                showToast(data.error, 'error');
                return;
            }
            
            const apiKey = data.key;
            
            // 显示测试中提示
            const endpointName = endpoint === 'chat' ? '对话' : 
                                endpoint === 'embeddings' ? '嵌入' : 
                                endpoint === 'models' ? '模型列表' : 
                                endpoint === 'rerank' ? '重排序' : '图片生成';
            showToast(`正在测试${endpointName}API,请稍候...`, 'info');
            
            if (endpoint === 'embeddings') {
                // 测试embeddings API
                fetch('/test-embeddings', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: apiKey,
                    }),
                })
                    .then(response => response.json())
                    .then(data => {
                        if (data.success) {
                            showToast('嵌入API测试成功！', 'success');
                        } else {
                            showToast(`嵌入API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        console.error('Error testing embeddings API:', error);
                        showToast('嵌入API测试失败', 'error');
                    });
            } else if (endpoint === 'images') {
                // 测试图片生成API
                fetch('/test-images', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: apiKey,
                    }),
                })
                    .then(response => response.json())
                    .then(data => {
                        if (data.success) {
                            showToast('图片生成API测试成功！', 'success');
                            // 检查响应中的images字段是否存在且为非空数组
                            if (data.response && data.response.images && 
                                Array.isArray(data.response.images) && 
                                data.response.images.length > 0) {
                            } else {
                                console.log('图片生成成功，但返回的图片数组为空或格式不正确');
                            }
                        } else {
                            showToast(`图片生成API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        showToast('图片生成API测试失败', 'error');
                    });
            } else if (endpoint === 'models') {
                // 测试模型列表API
                fetch('/test-models', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: apiKey,
                    }),
                })
                    .then(response => response.json())
                    .then(data => {
                        if (data.success) {
                            showToast('模型列表API测试成功!', 'success');
                            // 检查响应中的data字段是否存在且为非空数组
                            if (data.response && data.response.data && 
                                Array.isArray(data.response.data) && 
                                data.response.data.length > 0) {
                            } else {
                                console.log('模型列表API测试成功,但返回的模型数组为空或格式不正确');
                            }
                        } else {
                            showToast(`模型列表API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        console.error('Error testing models API:', error);
                        showToast('模型列表API测试失败', 'error');
                    });
            } else if (endpoint === 'rerank') {
                // 测试重排序API
                fetch('/test-rerank', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: apiKey,
                    }),
                })
                    .then(response => response.json())
                    .then(data => {
                        if (data.success) {
                            showToast('重排序API测试成功！', 'success');
                        } else {
                            showToast(`重排序API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        console.error('Error testing rerank API:', error);
                        showToast('重排序API测试失败', 'error');
                    });
            } else if (endpoint === 'chat') {
                // 测试chat API


                fetch(`/test-chat`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: apiKey,
                    }),
                })
                    .then(response => response.json())
                    .then(data => {
                        if (data.success) {
                            showToast('对话API测试成功！', 'success');
                        } else {
                            showToast(`对话API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        console.error('Error testing chat API:', error);
                        showToast('对话API测试失败', 'error');
                    });
            }
        })
}

// 启用 API 密钥
function enableKey(key) {
    fetch(`/keys/${key}/enable`, {
        method: 'POST',
    })
        .then(response => {
            if (!response.ok) {
                // 尝试解析错误消息
                return response.json().then(errorData => {
                    throw new Error(errorData.error || `启用密钥失败: ${response.status}`);
                }).catch(parseError => {
                    // 如果JSON解析失败，抛出原始错误
                    throw new Error(`启用密钥失败: ${response.status}`);
                });
            }
            return response.json();
        })
        .then(() => {
            // 显示成功消息
            showToast('API 密钥启用成功', 'success');
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
        })
        .catch(error => {
            console.error('Error enabling key:', error);
            showToast(error.message || '启用密钥失败', 'error');
        });
}

// 禁用 API 密钥
function disableKey(key) {
    fetch(`/keys/${key}/disable`, {
        method: 'POST',
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to disable key');
            }
            return response.json();
        })
        .then(() => {
            // 显示成功消息
            showToast('API 密钥禁用成功', 'success');
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
        })
        .catch(error => {
            console.error('Error disabling key:', error);
            showToast('禁用密钥失败', 'error');
        });
}

// 删除余额为0或负数的API密钥
function deleteZeroBalanceKeys() {
    // 获取当前展示的API密钥，用于进度条
    const keysContainer = document.getElementById('keys-container');
    let zeroBalanceKeys = [];
    
    if (keysContainer) {
        zeroBalanceKeys = Array.from(keysContainer.querySelectorAll('.key-item')).filter(item => {
            const balance = parseFloat(item.getAttribute('data-balance') || 0);
            return balance <= 0;
        });
    }
    
    // 显示进度条
    showProgressBar(`删除余额为0或负数的API密钥`, zeroBalanceKeys.length || 1);
    updateProgressBar(0, zeroBalanceKeys.length || 1, '准备删除中...');
    
    // 创建一个变量跟踪起始时间
    const startTime = new Date().getTime();
    
    fetch('/keys/zero-balance', {
        method: 'DELETE',
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Failed to delete zero balance keys');
        }
        return response.json();
    })
    .then(data => {
        // 计算处理时间
        const endTime = new Date().getTime();
        const duration = ((endTime - startTime) / 1000).toFixed(2);
        
        // 更新进度条到完成状态
        const deletedCount = data.deleted ? data.deleted.length : 0;
        updateProgressBar(
            zeroBalanceKeys.length || 1, 
            zeroBalanceKeys.length || 1, 
            `删除完成！成功删除 ${deletedCount} 个余额为0或负数的密钥，耗时: ${duration}秒`, 
            deletedCount > 0 ? 'success' : 'warning'
        );
        
        // 延迟隐藏进度条，让用户看到结果
        setTimeout(() => {
            hideProgressBar();
            
            // 显示吐司提示
            showToast(`成功删除 ${deletedCount} 个余额为0或负数的API密钥`, 'success');
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
        }, 1500);
    })
    .catch(error => {
        console.error('Error deleting zero balance keys:', error);
        
        // 更新进度条为错误状态
        updateProgressBar(
            0, 
            zeroBalanceKeys.length || 1, 
            `删除失败: ${error.message}`, 
            'error'
        );
        
        // 延迟隐藏进度条
        setTimeout(() => {
            hideProgressBar();
            
            // 显示吐司提示
            showToast('删除余额为0或负数的API密钥失败: ' + error.message, 'error');
        }, 2000);
    });
}

// 批量删除余额低于指定值的API密钥
function deleteLowBalanceKeys(threshold) {
    // 获取当前展示的API密钥，用于进度条
    const keysContainer = document.getElementById('keys-container');
    let allKeys = [];
    
    if (keysContainer) {
        allKeys = Array.from(keysContainer.querySelectorAll('.key-item')).filter(item => {
            const balance = parseFloat(item.getAttribute('data-balance') || 0);
            return balance < threshold;
        });
    }
    
    // 显示进度条
    showProgressBar(`删除余额低于 ${threshold} 的API密钥`, allKeys.length || 1);
    updateProgressBar(0, allKeys.length || 1, '准备删除中...');
    
    // 创建一个变量跟踪起始时间
    const startTime = new Date().getTime();
    
    // 发送请求删除低余额密钥
    fetch('/keys/low-balance/' + threshold, {
        method: 'DELETE',
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('删除低余额密钥失败');
        }
        return response.json();
    })
    .then(data => {
        // 计算处理时间
        const endTime = new Date().getTime();
        const duration = ((endTime - startTime) / 1000).toFixed(2);
        
        // 更新进度条到完成状态
        const deletedCount = data.deleted || 0;
        updateProgressBar(
            allKeys.length || 1, 
            allKeys.length || 1, 
            `删除完成！成功删除 ${deletedCount} 个密钥，耗时: ${duration}秒`, 
            deletedCount > 0 ? 'success' : 'warning'
        );
        
        // 延迟隐藏进度条，让用户看到结果
        setTimeout(() => {
            hideProgressBar();
            
            // 显示成功消息
            const deleteResult = document.getElementById('delete-result');
            if (deleteResult) {
                if (data.deleted_keys != null) {
                    deleteResult.innerHTML = '<div class="alert alert-success">' + data.message + '</div>';
                } else {
                    deleteResult.innerHTML = '<div class="alert alert-info">' + data.message + '</div>';
                }
            }
            
            // 显示吐司提示
            showToast(data.message || `成功删除 ${deletedCount} 个余额低于 ${threshold} 的密钥`, 'success');
            
            // 重新加载密钥列表和系统概要
            loadKeys();
            loadStats();
        }, 1500);
    })
    .catch(error => {
        console.error('Error deleting low balance keys:', error, threshold);
        
        // 更新进度条为错误状态
        updateProgressBar(
            0, 
            allKeys.length || 1, 
            `删除失败: ${error.message}`, 
            'error'
        );
        
        // 延迟隐藏进度条
        setTimeout(() => {
            hideProgressBar();
            
            // 显示错误消息
            const deleteResult = document.getElementById('delete-result');
            if (deleteResult) {
                deleteResult.innerHTML = '<div class="alert alert-danger">删除失败: ' + error.message + '</div>';
            }
            
            // 显示吐司提示
            showToast(`删除余额低于 ${threshold} 的密钥失败: ${error.message}`, 'error');
        }, 2000);
    });
}

// 启动自动更新
function startAutoUpdate() {
    
    // 清除所有现有计时器
    if (rateUpdateTimer) {
        clearInterval(rateUpdateTimer);
        rateUpdateTimer = null;
    }
    
    if (statsUpdateTimer) {
        clearInterval(statsUpdateTimer);
        statsUpdateTimer = null;
    }
    
    if (autoUpdateTimer) {
        clearInterval(autoUpdateTimer);
        autoUpdateTimer = null;
    }
    
    // 清除所有倒计时计时器
    if (rateUpdateCountdownTimer) {
        clearInterval(rateUpdateCountdownTimer);
        rateUpdateCountdownTimer = null;
    }
    
    if (statsUpdateCountdownTimer) {
        clearInterval(statsUpdateCountdownTimer);
        statsUpdateCountdownTimer = null;
    }
    
    if (keysUpdateCountdownTimer) {
        clearInterval(keysUpdateCountdownTimer);
        keysUpdateCountdownTimer = null;
    }

    // 设置速率监控更新定时器
    rateUpdateTimer = setInterval(() => {
        //console.log(`速率监控更新 (${RATE_REFRESH_INTERVAL}秒)`);
        loadCurrentRequestStats();
    }, RATE_REFRESH_INTERVAL * 1000);
    
    // 设置系统概要更新定时器
    statsUpdateTimer = setInterval(() => {
        //console.log(`系统概要更新 (${STATS_REFRESH_INTERVAL}秒)`);
        loadStats();
    }, STATS_REFRESH_INTERVAL * 1000);
    
    // 设置API密钥状态更新定时器
    autoUpdateTimer = setInterval(() => {
        
        // 记录当前选中的密钥和排序状态
        const selectedKeys = getSelectedKeys();
        const sortState = {
            field: currentSortField,
            direction: currentSortDirection
        };
        
        // 更新密钥列表
        fetch('/keys')
            .then(response => response.json())
            .then(data => {
                // 获取所有密钥
                const keys = data.keys || [];
                
                // 将密钥分为启用和禁用两组
                const enabledKeys = keys.filter(key => !key.disabled);
                const disabledKeys = keys.filter(key => key.disabled);
                
                // 合并，确保禁用的密钥始终在最后
                allKeys = [...enabledKeys, ...disabledKeys];
                
                // 检查是否没有密钥
                if (allKeys.length === 0) {
                    document.getElementById('keys-container').innerHTML = '<div class="alert alert-info">没有可用API 密钥, 请先添加API密钥</div>';
                    document.getElementById('keys-pagination').innerHTML = '';
                    
                    // 更新最后更新时间，但不启动倒计时
                    const now = new Date();
                    const timeStr = now.toLocaleTimeString();
                    document.getElementById('keys-last-update').textContent = `上次更新: ${timeStr} (已暂停更新)`;
                    
                    // 清除倒计时计时器
                    if (keysUpdateCountdownTimer) {
                        clearInterval(keysUpdateCountdownTimer);
                        keysUpdateCountdownTimer = null;
                    }
                    return;
                }
                
                // 恢复选中状态
                if (selectedKeys.length > 0) {
                    allKeys.forEach(key => {
                        if (selectedKeys.includes(key.key)) {
                            key.selected = true;
                        }
                    });
                }
                
                // 应用排序
                if (sortState.field) {
                    sortAllKeys(sortState.field, sortState.direction);
                } else {
                    renderKeysList();
                }
                
                // 加载完密钥列表后，更新当前使用的API密钥信息
                loadCurrentKeyInfo();
                
                // 更新最后更新时间
                const now = new Date();
                const timeStr = now.toLocaleTimeString();
                document.getElementById('keys-last-update').textContent = `上次更新: ${timeStr} (${AUTO_UPDATE_INTERVAL}秒后更新)`;
                
                // 开始倒计时
                startKeysUpdateCountdown(AUTO_UPDATE_INTERVAL);
            })
            .catch(error => {
                console.error('Error updating keys:', error);
            });
            
    }, AUTO_UPDATE_INTERVAL * 1000);
    
    // 立即执行一次更新
    loadCurrentRequestStats();
    loadStats();
}

// 在页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    //console.log("页面加载完成，开始初始化...");
    
    // 初始化Toast容器
    if (!document.getElementById('toast-container')) {
        const toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.className = 'toast-container';
        document.body.appendChild(toastContainer);
    }
    
    // 设置初始的更新时间显示
    const now = new Date();
    const timeStr = now.toLocaleTimeString();
    
    if (document.getElementById('stats-last-update')) {
        document.getElementById('stats-last-update').textContent = `上次更新: ${timeStr} (${STATS_REFRESH_INTERVAL}秒后更新)`;
    }
    
    if (document.getElementById('dashboard-last-update')) {
        document.getElementById('dashboard-last-update').textContent = `上次更新: ${timeStr} (${RATE_REFRESH_INTERVAL}秒后更新)`;
    }
    
    if (document.getElementById('keys-last-update')) {
        document.getElementById('keys-last-update').textContent = `上次更新: ${timeStr} (${AUTO_UPDATE_INTERVAL}秒后更新)`;
    }
    
    // 加载初始数据
    loadKeys();
    loadStats();
    loadCurrentRequestStats();
    
    // 初始化排序按钮
    initSortButtons();
    
    // 启动自动更新
    startAutoUpdate();
    
    // 更新API地址显示
    try {
        updateApiEndpoints();
    } catch (error) {
        console.error('更新API地址显示失败:', error);
    }
    
    // 加载常用模型
    loadTopModels();
    
    // 添加常用模型的样式
    const modelStyle = document.createElement('style');
    modelStyle.textContent = `
        .top-models-list {
            display: flex;
            flex-direction: column;
            gap: 10px;
        }
        
        .top-model-item {
            padding: 10px;
            border: 1px solid #eee;
            border-radius: 5px;
            background-color: #f9f9f9;
        }
        
        .model-name {
            font-weight: bold;
            margin-bottom: 5px;
            word-break: break-all;
        }
        
        .model-info {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
    `;
    document.head.appendChild(modelStyle);
    
    // 添加检查余额按钮事件
    document.getElementById('check-balance-btn').addEventListener('click', function() {
        const key = document.getElementById('key').value;
        if (!key) {
            showToast('请输入 API 密钥', 'error');
            return;
        }
        
        checkKeyBalance(key);
    });
    
    // 添加单个密钥表单提交事件
    document.getElementById('add-key-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const key = document.getElementById('key').value;
        const balance = document.getElementById('balance').value;
        
        // 检查是否是特定链接
        if (key.trim() === 'https://sili-api.killerbest.com' || key.trim().startsWith('https://sili-api.killerbest.com')) {
            // 如果是特定链接，调用检查余额函数处理
            checkKeyBalance(key).catch(error => {
                console.error('处理特定链接失败:', error);
                // 错误已在checkKeyBalance中处理，这里不需要额外处理
            });
        } else {
            // 如果是普通API密钥，正常添加
            addKey(key, balance);
        }
    });
    
    // 添加批量密钥表单提交事件
    document.getElementById('batch-add-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const keysInput = document.getElementById('batch-keys').value;
        const balance = document.getElementById('batch-balance').value;
        
        const keys = parseKeys(keysInput);
        
        if (keys.length === 0) {
            showToast('请输入至少一个 API 密钥', 'error');
            return;
        }
        
        batchAddKeys(keys, balance);
    });
    
    // 添加批量删除表单提交事件
    document.getElementById('batch-delete-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const threshold = document.getElementById('delete-balance-threshold').value;
        const confirmed = document.getElementById('confirm-batch-delete').checked;
        
        if (!confirmed) {
            showToast('请确认删除操作', 'error');
            return;
        }
        
        if (threshold < 0) {
            showToast('余额阈值必须大于等于0', 'error');
            return;
        }
        
        deleteLowBalanceKeys(threshold);
    });
    
    // 添加刷新按钮事件
    document.getElementById('refresh-keys').addEventListener('click', function() {
        // 检查按钮是否被禁用
        if (this.disabled) {
            return;
        }
        refreshAllKeysBalance(false); // 明确使用非静默模式
    });
    
    // 添加查看日志按钮事件
    document.getElementById('view-logs').addEventListener('click', function() {
        showLogViewer();
    });
    
    // 添加清空日志按钮事件
    document.getElementById('clear-logs').addEventListener('click', function() {
        clearLogs();
    });
    
    // 添加关闭日志查看器事件
    document.getElementById('log-close').addEventListener('click', function() {
        hideLogViewer();
    });
    
    // 添加导出密钥按钮事件
    const exportKeysBtn = document.getElementById('export-keys');
    if (exportKeysBtn) {
        exportKeysBtn.addEventListener('click', function() {
            exportKeys();
        });
    } else {
        console.error('导出密钥按钮未找到');
    }

    // 添加导出格式选择事件
    const exportNewlineBtn = document.getElementById('export-newline-btn');
    if (exportNewlineBtn) {
        exportNewlineBtn.addEventListener('click', function() {
            exportKeysWithFormat('newline');
            bootstrap.Modal.getInstance(document.getElementById('export-format-modal')).hide();
        });
    }

    const exportCommaBtn = document.getElementById('export-comma-btn');
    if (exportCommaBtn) {
        exportCommaBtn.addEventListener('click', function() {
            exportKeysWithFormat('comma');
            bootstrap.Modal.getInstance(document.getElementById('export-format-modal')).hide();
        });
    }
    
    // 添加单独使用选中密钥按钮事件
    document.getElementById('use-single-key').addEventListener('click', function() {
        const selectedKeys = getSelectedKeys();
        if (selectedKeys.length !== 1) {
            showToast('请选择一个 API 密钥', 'error');
            return;
        }
        
        setKeyMode('single', selectedKeys);
    });
    
    // 添加轮询所有密钥按钮事件
    document.getElementById('use-all-keys').addEventListener('click', function() {
        setKeyMode('all');
    });
    
    // 添加轮询选中密钥按钮事件
    document.getElementById('use-selected-keys').addEventListener('click', function() {
        const selectedKeys = getSelectedKeys();
        
        if (selectedKeys.length === 0) {
            showToast('请选择至少一个 API 密钥', 'error');
            return;
        }
        
        if (selectedKeys.length < 2) {
            showToast('轮询选中模式需要至少选择两个 API 密钥', 'error');
            return;
        }
        
        setKeyMode('selected', selectedKeys);
        
        // 立即更新复选框状态
        document.querySelectorAll('.key-select').forEach(checkbox => {
            checkbox.checked = selectedKeys.includes(checkbox.dataset.key);
        });
    });
    
    // 添加清空已保存密钥按钮事件（如果元素存在）
    const clearSavedKeysBtn = document.getElementById('clear-saved-keys');
    if (clearSavedKeysBtn) {
        clearSavedKeysBtn.addEventListener('click', clearSavedKeys);
    }
    
    // 添加删除余额为0的密钥按钮事件（如果元素存在）
    const deleteZeroBalanceKeysBtn = document.getElementById('delete-zero-balance-keys');
    if (deleteZeroBalanceKeysBtn) {
        deleteZeroBalanceKeysBtn.addEventListener('click', function() {
            // 确认删除
            if (confirm('确定要删除所有余额小于或等于0的API密钥吗？')) {
                deleteZeroBalanceKeys();
            }
        });
    }
    
    // 添加从文件导入按钮事件
    const importFileBtn = document.getElementById('import-file-btn');
    if (importFileBtn) {
        importFileBtn.addEventListener('click', function() {
            const fileInput = document.getElementById('import-file');
            if (!fileInput || !fileInput.files || fileInput.files.length === 0) {
                showToast('请先选择要导入的文件', 'warning');
                return;
            }
            
            const file = fileInput.files[0];
            if (file.size > 1024 * 1024) { // 限制文件大小最大1MB
                showToast('文件过大，请选择小于1MB的文件', 'warning');
                return;
            }
            
            importKeysFromFile(file);
        });
    }
    
    // 添加文件选择监听
    const importFileInput = document.getElementById('import-file');
    if (importFileInput) {
        importFileInput.addEventListener('change', function() {
            if (this.files && this.files.length > 0) {
                const importFileBtn = document.getElementById('import-file-btn');
                if (importFileBtn) {
                    // 更新按钮样式
                    importFileBtn.classList.remove('btn-outline-secondary');
                    importFileBtn.classList.add('btn-outline-primary');
                    // 自动触发导入
                    setTimeout(() => {
                        importKeysFromFile(this.files[0]);
                    }, DOM_CLEANUP_DELAY);
                }
            }
        });
    }
    
    // 加载常用模型
    loadTopModels();
});

// 在页面关闭或切换时清除定时器
window.addEventListener('beforeunload', function() {
    if (autoUpdateTimer) {
        clearInterval(autoUpdateTimer);
    }
    if (statsUpdateTimer) {
        clearInterval(statsUpdateTimer);
    }
    if (rateUpdateTimer) {
        clearInterval(rateUpdateTimer);
    }
});

// 排序函数 - 用于对allKeys数组进行排序
function sortAllKeys(field, direction) {
    
    // 检查是否没有密钥
    if (allKeys.length === 0) {
        return;
    }
    
    // 先获取选中的密钥
    const selectedKeys = getSelectedKeys();
    
    // 将密钥分为启用和禁用两组
    const enabledKeys = allKeys.filter(key => !key.disabled);
    const disabledKeys = allKeys.filter(key => key.disabled);
    
    // 对启用的密钥进行排序
    enabledKeys.sort((a, b) => {
        // 如果是选中的密钥，放在前面
        const aSelected = selectedKeys.includes(a.key);
        const bSelected = selectedKeys.includes(b.key);
        
        if (aSelected && !bSelected) return -1;
        if (!aSelected && bSelected) return 1;
        
        // 如果都是选中的或都不是选中的，按照指定字段排序
        if (!field) return 0;
        
        let valueA = 0, valueB = 0;
        
        switch(field) {
            case 'score':
                valueA = parseFloat(a.score || 0);
                valueB = parseFloat(b.score || 0);
                break;
            case 'balance':
                valueA = parseFloat(a.balance || 0);
                valueB = parseFloat(b.balance || 0);
                break;
            case 'success_rate':
                valueA = parseFloat(a.success_rate || 0);
                valueB = parseFloat(b.success_rate || 0);
                break;
            case 'usage':
                valueA = parseInt(a.total_calls || 0);
                valueB = parseInt(b.total_calls || 0);
                break;
            case 'rpm':
                valueA = parseInt(a.rpm || 0);
                valueB = parseInt(b.rpm || 0);
                break;
            case 'tpm':
                valueA = parseInt(a.tpm || 0);
                valueB = parseInt(b.tpm || 0);
                break;
            default:
                return 0;
        }
        
        if (direction === 'asc') {
            return valueA - valueB;
        } else {
            return valueB - valueA;
        }
    });
    
    // 合并排序后的启用密钥和禁用密钥（禁用的放在最后）
    allKeys = [...enabledKeys, ...disabledKeys];
    
    // 重新渲染密钥列表
    renderKeysList();
}

// 更新排序按钮状态
function updateSortButtons() {
    document.querySelectorAll('.sort-btn').forEach(btn => {
        const field = btn.dataset.sort;
        btn.classList.remove('active', 'asc');
        if (field === currentSortField) {
            btn.classList.add('active');
            if (currentSortDirection === 'asc') {
                btn.classList.add('asc');
            }
        }
    });
}

// 初始化排序按钮事件监听
function initSortButtons() {
    // 先移除所有已有的事件监听器，避免重复绑定
    document.querySelectorAll('.sort-btn').forEach(btn => {
        const newBtn = btn.cloneNode(true);
        btn.parentNode.replaceChild(newBtn, btn);
    });
    
    // 重新绑定事件监听器
    document.querySelectorAll('.sort-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const field = this.dataset.sort;
            if (field === currentSortField) {
                currentSortDirection = currentSortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                currentSortField = field;
                currentSortDirection = 'desc';
            }
            updateSortButtons();
            
            // 对所有密钥进行排序，然后重新渲染
            sortAllKeys(currentSortField, currentSortDirection);
        });
    });
    
    // 更新排序按钮状态
    updateSortButtons();
}

// 应用排序并更新显示
function applySorting() {
    const container = document.getElementById('keys-container');
    if (!container) return;
    
    const keyElements = Array.from(container.querySelectorAll('.key-item'));
    if (keyElements.length === 0) return;
    
    // 首先将选中的密钥移到最前面
    const selectedKeyElements = keyElements.filter(el => {
        const checkbox = el.querySelector('.key-checkbox');
        return checkbox && checkbox.checked;
    });
    const unselectedKeyElements = keyElements.filter(el => {
        const checkbox = el.querySelector('.key-checkbox');
        return !checkbox || !checkbox.checked;
    });
    
    // 对未选中的密钥进行排序
    const sortedUnselectedKeys = currentSortField ? 
        sortElementsByAttribute(unselectedKeyElements, currentSortField, currentSortDirection) : 
        unselectedKeyElements;
    
    // 合并选中和未选中的密钥
    const allSortedKeys = [...selectedKeyElements, ...sortedUnselectedKeys];
    
    // 清空容器并重新添加排序后的元素
    container.innerHTML = '';
    allSortedKeys.forEach(el => container.appendChild(el));
    
    // 重新绑定事件监听器
    bindKeyEvents();
}

// 按属性对DOM元素进行排序
function sortElementsByAttribute(elements, attrName, direction) {
    return [...elements].sort((a, b) => {
        let valueA, valueB;
        
        // 根据属性名称获取相应的值
        if (attrName === 'score' || attrName === 'balance' || attrName === 'success_rate') {
            valueA = parseFloat(a.dataset[attrName] || 0);
            valueB = parseFloat(b.dataset[attrName] || 0);
        } else {
            valueA = parseInt(a.dataset[attrName.replace('_', '-')] || 0);
            valueB = parseInt(b.dataset[attrName.replace('_', '-')] || 0);
        }
        
        // 根据排序方向进行比较
        if (direction === 'asc') {
            return valueA - valueB;
        } else {
            return valueB - valueA;
        }
    });
}

// 绑定密钥相关的事件监听器
function bindKeyEvents() {
    // 添加复选框事件
    document.querySelectorAll('.key-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            applySorting();
        });
    });
    
    // 添加其他事件监听器...（保持原有的其他事件绑定代码）
}

// 导出所有API密钥
function exportKeys() {
    // 显示导出选择模态框
    const exportModal = document.getElementById('export-format-modal');
    if (exportModal) {
        const bsModal = new bootstrap.Modal(exportModal);
        bsModal.show();
    } else {
        console.error('导出格式选择模态框未找到，使用默认格式导出');
        exportKeysWithFormat('newline'); // 默认使用换行符分隔
    }
}

// 根据指定格式导出密钥
function exportKeysWithFormat(format) {
    // 获取所有密钥
    fetch('/keys')
        .then(response => {
            console.log('获取密钥API响应:', response.status);
            if (!response.ok) {
                throw new Error(`获取密钥失败，状态码: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            
            if (!data.keys || data.keys.length === 0) {
                showToast('没有可导出的API密钥', 'warning');
                return;
            }

            // 提取所有密钥
            const keys = data.keys.map(k => k.key);
            
            // 根据格式选择分隔符
            let keyText, fileName;
            if (format === 'comma') {
                keyText = keys.join(',');
                fileName = 'api_keys_comma.txt';
            } else {
                keyText = keys.join('\n');
                fileName = 'api_keys.txt';
            }

            // 创建blob对象
            const blob = new Blob([keyText], { type: 'text/plain' });
            
            // 创建下载链接
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = fileName;
            
            // 触发下载
            document.body.appendChild(a);
            a.click();
            
            // 清理
            setTimeout(() => {
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
            }, DOM_CLEANUP_DELAY);

            showToast(`已导出 ${keys.length} 个API密钥`, 'success');
        })
        .catch(error => {
            console.error('导出密钥失败:', error);
            showToast('导出密钥失败，请查看控制台了解详情', 'danger');
        });
}

// 从文件导入API密钥
function importKeysFromFile(file) {
    if (!file) {
        showToast('请选择文件', 'error');
        return;
    }

    // 显示进度条
    showProgressBar('正在导入API密钥...', 100);
    updateProgressBar(10, 100, '正在读取文件...');

    const reader = new FileReader();
    reader.onload = (e) => {
        try {
            updateProgressBar(30, 100, '正在处理文件内容...');
            const content = e.target.result;
            const keys = parseKeys(content);
            
            if (keys.length === 0) {
                updateProgressBar(50, 100, '没有找到有效的API密钥', 'error');
                setTimeout(() => {
                    hideProgressBar();
                    showToast('没有找到有效的API密钥', 'error');
                }, 1500);
                return;
            }
            
            updateProgressBar(70, 100, `已解析出 ${keys.length} 个密钥...`);
            
            // 获取余额值
            const balanceInput = document.getElementById('batch-balance');
            const balance = balanceInput ? parseFloat(balanceInput.value) || 0 : 0;
            
            // 更新批量添加区域的文本框
            const batchKeysInput = document.getElementById('batch-keys');
            if (batchKeysInput) {
                batchKeysInput.value = keys.join('\n');
                updateProgressBar(90, 100, `成功导入 ${keys.length} 个密钥到输入框！`);
            }
            
            updateProgressBar(100, 100, `成功解析 ${keys.length} 个密钥！`, 'success');
            
            setTimeout(() => {
                hideProgressBar();
                showToast(`成功导入 ${keys.length} 个密钥到输入框，请点击"批量添加"按钮完成添加`, 'success');
                
                // 如果有密钥，自动聚焦到批量添加按钮
                const batchAddBtn = document.querySelector('#batch-add-form button[type="submit"]');
                if (batchAddBtn && keys.length > 0) {
                    batchAddBtn.focus();
                }
            }, 1000);
            
            // 清空文件输入框
            const fileInput = document.getElementById('import-file');
            if (fileInput) {
                fileInput.value = '';
            }
        } catch (error) {
            console.error('Error parsing file:', error);
            updateProgressBar(0, 100, `文件解析失败: ${error.message}`, 'error');
            setTimeout(() => {
                hideProgressBar();
                showToast(`文件解析失败: ${error.message}`, 'error');
            }, 1500);
        }
    };
    
    reader.onerror = () => {
        updateProgressBar(0, 100, '文件读取失败', 'error');
        setTimeout(() => {
            hideProgressBar();
            showToast('文件读取失败', 'error');
        }, 1500);
    };
    
    reader.readAsText(file);
}

// 显示进度条
function showProgressBar(title = '处理中...', total = 100) {
    const progressOverlay = document.getElementById('progress-overlay');
    const progressTitle = document.getElementById('progress-title');
    const progressBar = document.getElementById('progress-bar');
    const progressCurrent = document.getElementById('progress-current');
    const progressPercentage = document.getElementById('progress-percentage');
    const progressTotal = document.getElementById('progress-total');
    const progressStatus = document.getElementById('progress-status');
    
    progressTitle.textContent = title;
    progressBar.style.width = '0%';
    progressBar.className = 'progress-bar'; // 重置类名
    progressCurrent.textContent = '0';
    progressPercentage.textContent = '0%';
    progressTotal.textContent = total;
    progressStatus.textContent = '初始化中...';
    
    progressOverlay.style.display = 'flex';
}

// 更新进度条
function updateProgressBar(current, total, status = '', type = '') {
    const progressBar = document.getElementById('progress-bar');
    const progressCurrent = document.getElementById('progress-current');
    const progressPercentage = document.getElementById('progress-percentage');
    const progressTotal = document.getElementById('progress-total');
    const progressStatus = document.getElementById('progress-status');
    
    // 确保数值不为空
    current = current || 0;
    total = total || 1; // 防止除以零
    
    const percentage = Math.round((current / total) * 100);
    
    // 更新进度条和显示的数值
    progressBar.style.width = `${percentage}%`;
    progressCurrent.textContent = current;
    progressTotal.textContent = total;
    progressPercentage.textContent = `${percentage}%`;
    
    if (status) {
        progressStatus.textContent = status;
    }
    
    // 更新进度条类型
    if (type) {
        progressBar.className = `progress-bar ${type}`;
    }
}

// 隐藏进度条
function hideProgressBar() {
    const progressOverlay = document.getElementById('progress-overlay');
    progressOverlay.style.display = 'none';
}

// 加载常用模型
function loadTopModels() {
    
    fetch('/models/top')
        .then(response => response.json())
        .then(data => {
            if (data.success && data.models && data.models.length > 0) {
                const container = document.getElementById('top-models-container');
                let html = '<div class="top-models-list">';
                
                // 遍历模型
                data.models.forEach(model => {
                    html += `
                    <div class="top-model-item">
                        <div class="model-name">${model.id}</div>
                        <div class="model-info">
                            <span class="badge bg-info">调用次数: ${model.call_count}</span>
                            <button class="btn btn-sm btn-outline-primary copy-model-btn" data-model="${model.id}">
                                <i class="bi bi-clipboard"></i> 复制
                            </button>
                        </div>
                    </div>`;
                });
                
                html += '</div>';
                container.innerHTML = html;
                
                // 添加复制功能
                document.querySelectorAll('.copy-model-btn').forEach(btn => {
                    btn.addEventListener('click', function() {
                        const modelId = this.getAttribute('data-model');
                        navigator.clipboard.writeText(modelId)
                            .then(() => {
                                showToast('模型ID已复制到剪贴板', 'success', 1500);
                            })
                            .catch(err => {
                                showToast('请手动复制模型ID', 'error', 1500);
                                console.error('复制失败:', err);
                            });
                    });
                });
                
                // 更新时间
                document.getElementById('top-models-last-update').textContent = '上次更新: ' + new Date().toLocaleTimeString();
            } else {
                document.getElementById('top-models-container').innerHTML = '<div class="alert alert-info">暂无常用模型数据</div>';
            }
        })
        .catch(error => {
            console.error('获取常用模型失败:', error);
            document.getElementById('top-models-container').innerHTML = '<p>获取常用模型失败</p>';
        });
}

// 添加常用模型的样式
document.addEventListener('DOMContentLoaded', function() {
    // 创建样式元素
    const style = document.createElement('style');
    style.textContent = `
        .top-models-list {
            display: flex;
            flex-direction: column;
            gap: 10px;
        }
        
        .top-model-item {
            padding: 10px;
            border: 1px solid #eee;
            border-radius: 5px;
            background-color: #f9f9f9;
        }
        
        .model-name {
            font-weight: bold;
            margin-bottom: 5px;
            word-break: break-all;
        }
        
        .model-info {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
    `;
    
    // 添加到文档头部
    document.head.appendChild(style);
});