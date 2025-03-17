/**
 @author: Hanhai
 @since: 2025/3/16 21:56:10
 @desc:
 **/

// 全局变量已在HTML中定义
let currentPage = 1; // 当前页码
let allKeys = []; // 所有密钥
let selectedKeyId = null; // 当前选中的密钥ID
let autoUpdateTimer = null; // 自动更新定时器
let statsUpdateTimer = null; // 统计信息更新定时器
let rateUpdateTimer = null; // 速率监控更新定时器

// 本地存储键名
const STORAGE_KEY = 'guiji_saved_api_keys';

// 排序相关变量
let currentSortField = '';
let currentSortDirection = 'desc';
let selectedKeys = new Set();

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

// 一键添加所有保存的API密钥
function addAllSavedKeys() {
    const savedKeys = getSavedKeys();
    if (savedKeys.length === 0) {
        showToast('没有已保存的API密钥', 'info');
        return;
    }
    
    // 获取当前系统中的密钥
    fetch('/keys')
        .then(response => response.json())
        .then(data => {
            const existingKeys = data.keys || [];
            const keysToAdd = [];
            
            // 找出需要添加的密钥
            savedKeys.forEach(key => {
                const keyExists = existingKeys.some(k => k.key === key);
                if (!keyExists) {
                    keysToAdd.push(key);
                }
            });
            
            if (keysToAdd.length === 0) {
                showToast('所有已保存的密钥都已添加到系统中', 'info');
                return;
            }
            
            // 批量添加密钥
            fetch('/keys/batch', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    keys: keysToAdd,
                    balance: 0, // 余额设为0，系统会自动检查
                }),
            })
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Failed to add keys');
                    }
                    return response.json();
                })
                .then(data => {
                    showToast(`成功添加 ${data.added} 个密钥`, 'success');
                    loadKeys();
                    loadStats();
                })
                .catch(error => {
                    console.error('Error adding keys:', error);
                    showToast('添加密钥失败', 'error');
                });
        });
}

// 加载 API 密钥列表
function loadKeys() {
    // 显示刷新按钮的加载状态
    document.getElementById('refresh-spinner').style.display = 'inline-block';
    
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
            
            // 如果有排序字段，应用排序
            if (currentSortField) {
                sortAllKeys(currentSortField, currentSortDirection);
            } else {
                renderKeysList();
            }
            
            // 加载完密钥列表后，更新当前使用的API密钥信息
            loadCurrentKeyInfo();
            
            // 处理自动添加保存的密钥
            handleSavedKeys(allKeys);
        })
        .catch(error => {
            console.error('Error loading keys:', error);
            document.getElementById('keys-container').innerHTML = '<p>加载失败</p>';
            document.getElementById('keys-pagination').innerHTML = '';
        })
        .finally(() => {
            // 隐藏刷新按钮的加载状态
            document.getElementById('refresh-spinner').style.display = 'none';
        });
}

// 处理自动添加保存的密钥
function handleSavedKeys(existingKeys) {
    const savedKeys = getSavedKeys();
    if (savedKeys.length === 0) return;
    
    const keysToAdd = [];
    
    // 找出需要添加的密钥
    savedKeys.forEach(key => {
        const keyExists = existingKeys.some(k => k.key === key);
        if (!keyExists) {
            keysToAdd.push(key);
        }
    });
    
    // 如果有需要添加的密钥，使用批量添加API
    if (keysToAdd.length > 0) {
        fetch('/keys/batch', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                keys: keysToAdd,
                balance: 0 // 余额设为0，系统会自动检查
            }),
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to add keys');
            }
            return response.json();
        })
        .then(data => {
            if (data.added > 0) {
                showToast(`已自动添加 ${data.added} 个保存的API密钥`, 'success');
                // 重新加载密钥列表和统计信息
                loadKeys();
                loadStats();
            }
        })
        .catch(error => {
            console.error('Error adding saved keys:', error);
        });
    }
}

// 渲染密钥列表
function renderKeysList() {
    const keysContainer = document.getElementById('keys-container');
    
    if (allKeys.length === 0) {
        keysContainer.innerHTML = '<p>没有 API 密钥</p>';
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
                    <div class="key-name">
                        <input type="checkbox" class="form-check-input key-checkbox key-select" data-key="${key.key}" ${key.disabled ? 'disabled' : ''} ${isSelected ? 'checked' : ''}>
                        <span class="ms-2">${maskedKey}</span>
                    </div>
                    <div class="key-balance-container">
                        <span class="key-score" data-score="${key.score || 0}">${key.score ? key.score.toFixed(2) : '0.00'}</span>
                        <span class="ms-3">余额: <span class="key-balance" data-balance="${key.balance || 0}">${key.balance.toFixed(2)}</span></span>
                    </div>
                    <div class="key-stats">
                        <span class="key-stat" data-usage="${key.total_calls || 0}">调用: ${key.total_calls}</span>
                        <span class="key-stat" data-success-rate="${key.success_rate || 0}">成功率: ${successRatePercent.toFixed(1)}%</span>
                        <span class="key-stat rpm-stat" data-rpm="${key.rpm || 0}">RPM: <span class="rpm-value">${key.rpm || 0}</span></span>
                        <span class="key-stat tpm-stat" data-tpm="${key.tpm || 0}">TPM: <span class="tpm-value">${key.tpm || 0}</span></span>
                    </div>
                    <div class="key-actions-container">
                        <div class="api-buttons-container">
                            <div class="form-check form-switch d-inline-block me-2">
                                <input class="form-check-input toggle-key-status" type="checkbox" role="switch" data-key="${key.key}" ${!key.disabled ? 'checked' : ''}>
                            </div>
                            <button class="copy-api-btn" data-key="${key.key}">复制</button>
                            <button class="check-api-btn" data-key="${key.key}">检测</button>
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
            
            if (confirm('确定要删除这个API密钥吗？')) {
                deleteKey(key);
            }
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
                }, 1500);
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
            this.textContent = '检测中...';
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
    
    // 页码按钮
    for (let i = 1; i <= totalPages; i++) {
        paginationHtml += `
            <li class="page-item ${i === currentPage ? 'active' : ''}">
                <a class="page-link" href="#" data-page="${i}">${i}</a>
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

// 加载统计信息
function loadStats() {
    fetch('/stats')
        .then(response => response.json())
        .then(data => {
            const statsContainer = document.getElementById('stats-container');
            
            // 计算成功率百分比
            const successRatePercent = data.avg_success_rate * 100;
            
            let html = `
                <div class="row">
                    <div class="col-6">
                        <p>总密钥数:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.total_keys}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>活跃密钥:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.active_keys}</strong></p>
                    </div>
                </div>
                <div class="row">
                    <div class="col-6">
                        <p>禁用密钥:</p>
                    </div>
                    <div class="col-6 text-end">
                        <p><strong>${data.disabled_keys}</strong></p>
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
                        <p>活跃密钥余额:</p>
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
                        <p>成功调用:</p>
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
            
            // 更新最后刷新时间
            const now = new Date();
            const timeStr = now.toLocaleTimeString();
            document.getElementById('stats-last-update').textContent = `上次更新: ${timeStr}`;
        })
        .catch(error => {
            console.error('Error loading stats:', error);
            document.getElementById('stats-container').innerHTML = '<p>加载失败</p>';
        });
}

// 检查 API 密钥余额
function checkKeyBalance(key) {
    // 显示加载状态
    const checkBtn = document.getElementById('check-balance-btn');
    const balanceResult = document.getElementById('balance-result');
    
    if (checkBtn) {
        checkBtn.disabled = true;
        checkBtn.innerHTML = '<span class="spinner-border" role="status" aria-hidden="true"></span>检查中...';
    }
    if (balanceResult) {
        balanceResult.style.display = 'none';
    }
    
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
                throw new Error('Failed to check balance');
            }
            return response.json();
        })
        .then(data => {
            // 显示余额
            if (balanceResult) {
                balanceResult.textContent = `余额: ${data.balance.toFixed(2)}`;
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
                balanceResult.textContent = '检查余额失败';
                balanceResult.style.display = 'block';
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
                throw new Error('Failed to check API key');
            }
            return response.json();
        })
        .then(data => {
            // 显示API密钥可用性
            showToast(`API密钥可用，余额: ${data.balance.toFixed(2)}`, 'success');
            return data;
        })
        .catch(error => {
            console.error('Error checking API key:', error);
            showToast('API密钥不可用，请检查密钥是否有效', 'error');
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

    fetch('/keys', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            key: key,
            balance: parseFloat(balance),
        }),
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to add key');
            }
            return response.json();
        })
        .then(data => {
            // 保存API密钥到本地存储
            saveKeyToLocalStorage(key);
            
            // 重新加载密钥列表和统计信息
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
            showToast('添加密钥失败', 'error');
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

    // 显示进度提示
    showToast(`开始添加 ${keys.length} 个密钥，请稍候...`, 'info');
    
    // 使用批量添加 API
    fetch('/keys/batch', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            keys: keys,
            balance: parseFloat(balance),
        }),
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to add keys');
            }
            return response.json();
        })
        .then(data => {
            // 保存所有API密钥到本地存储
            keys.forEach(key => saveKeyToLocalStorage(key));
            
            // 显示结果
            showToast(`批量添加完成！成功添加 ${data.added} 个密钥`, 'success');
            
            // 重新加载密钥列表和统计信息
            loadKeys();
            loadStats();
            
            // 清空表单
            document.getElementById('batch-keys').value = '';
            document.getElementById('batch-balance').value = '0';
        })
        .catch(error => {
            console.error('Error adding keys:', error);
            showToast('批量添加密钥失败', 'error');
        });
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
            
            // 重新加载密钥列表和统计信息
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
    document.querySelectorAll('.key-select:checked').forEach(checkbox => {
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
        return '******';
    }
    return key.substring(0, 6) + '******';
}

// 格式化日期
function formatDate(dateStr) {
    if (dateStr === 'Never') {
        return '从未';
    }
    
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN');
}

// 解析批量输入的密钥
function parseKeys(input) {
    // 首先按换行符分割
    let keys = input.split(/\n/);
    
    // 然后处理每一行，按逗号分割
    let result = [];
    keys.forEach(line => {
        // 移除行首行尾的空白字符
        line = line.trim();
        if (line) {
            // 按逗号分割并添加到结果中
            const lineKeys = line.split(',');
            lineKeys.forEach(k => {
                const trimmedKey = k.trim();
                if (trimmedKey) {
                    result.push(trimmedKey);
                }
            });
        }
    });
    
    // 去重
    return [...new Set(result)];
}

// 显示Toast提示
function showToast(message, type = 'info', duration = 3000) {
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
        }, 300);
    });
    
    // 自动关闭
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => {
            if (toastContainer.contains(toast)) {
                toastContainer.removeChild(toast);
            }
        }, 300);
    }, duration);
}

// 刷新所有API密钥的余额和状态
function refreshAllKeysBalance(silent = false) {
    // 显示刷新按钮的加载状态（仅在非静默模式下）
    const refreshSpinner = document.getElementById('refresh-spinner');
    if (refreshSpinner && !silent) {
        refreshSpinner.style.display = 'inline-block';
    }
    
    // 获取所有密钥
    fetch('/keys')
        .then(response => response.json())
        .then(data => {
            const keys = data.keys || [];
            if (keys.length === 0) {
                if (!silent) {
                    showToast('没有API密钥可刷新', 'info');
                }
                return;
            }
            
            // 显示开始刷新的提示（仅在非静默模式下）
            if (!silent) {
                showToast(`开始检查 ${keys.length} 个密钥的状态，请稍候...`, 'info');
            }
            
            // 创建一个Promise数组，用于并行检查所有密钥
            const promises = keys.map(key => 
                fetch('/keys/check', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        key: key.key,
                    }),
                })
                .then(response => {
                    if (!response.ok) {
                        return { key: key.key, success: false };
                    }
                    return response.json().then(data => ({ key: key.key, success: true, balance: data.balance }));
                })
                .catch(() => ({ key: key.key, success: false }))
            );
            
            // 等待所有请求完成
            return Promise.all(promises);
        })
        .then(results => {
            if (!results) return;
            
            // 统计成功和失败的数量
            const successful = results.filter(r => r.success).length;
            const failed = results.length - successful;
            
            // 显示结果（仅在非静默模式下）
            if (!silent) {
                showToast(`状态检查完成！成功: ${successful}, 失败: ${failed}`, 'success');
            }
            
            // 重新加载密钥列表和统计信息
            loadKeys();
            loadStats();
            loadCurrentRequestStats();
        })
        .catch(error => {
            console.error('Error refreshing balances:', error);
            if (!silent) {
                showToast('刷新状态失败', 'error');
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
let keyInfoDebounceTimer = null;
function loadCurrentKeyInfo() {
    // 清除之前的定时器
    if (keyInfoDebounceTimer) {
        clearTimeout(keyInfoDebounceTimer);
    }
    
    // 设置新的定时器，延迟300毫秒执行
    keyInfoDebounceTimer = setTimeout(() => {
        fetch('/keys/mode')
            .then(response => response.json())
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
                document.querySelectorAll('.key-select').forEach(checkbox => {
                    checkbox.checked = false;
                });
                
                // 根据当前模式和密钥更新复选框状态
                if (mode === 'single' || mode === 'selected') {
                    keys.forEach(key => {
                        const checkbox = document.querySelector(`.key-select[data-key="${key}"]`);
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
                document.getElementById('current-key-content').innerHTML = '<p>加载失败</p>';
            });
    }, 300);
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
                }, 1500);
                
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
            setTimeout(testNext, 1000);
        } else {
            // 所有接口测试完成
            setTimeout(() => {
                showToast('所有API接口测试完成!', 'success');
            }, 1000);
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
                                console.log('生成的图片URL:', data.response.images[0].url);
                            } else {
                                console.log('图片生成成功，但返回的图片数组为空或格式不正确');
                            }
                        } else {
                            showToast(`图片生成API测试失败: ${data.error}`, 'error');
                        }
                    })
                    .catch(error => {
                        console.error('Error testing images API:', error);
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
                                console.log('模型数量:', data.response.data.length);
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
                const baseUrl = window.location.origin;
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
                throw new Error('Failed to enable key');
            }
            return response.json();
        })
        .then(() => {
            // 显示成功消息
            showToast('API 密钥启用成功', 'success');
            
            // 重新加载密钥列表和统计信息
            loadKeys();
            loadStats();
        })
        .catch(error => {
            console.error('Error enabling key:', error);
            showToast('启用密钥失败', 'error');
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
            
            // 重新加载密钥列表和统计信息
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
            // 显示成功消息
            showToast(`成功删除 ${data.deleted.length} 个余额小于或等于0的API密钥`, 'success');
            
            // 重新加载密钥列表和统计信息
            loadKeys();
            loadStats();
        })
        .catch(error => {
            console.error('Error deleting zero balance keys:', error);
            showToast('删除余额小于或等于0的API密钥失败', 'error');
        });
}

// 加载当前请求速率
function loadCurrentRequestStats() {
    fetch('/request-stats/current')
        .then(response => response.json())
        .then(data => {
            console.log("获取到请求统计数据:", data); // 添加日志，查看返回的数据
            
            // 更新全局速率监控的数值
            document.getElementById('rpm-value').textContent = data.rpm || 0;
            document.getElementById('tpm-value').textContent = data.tpm || 0;
            
            // 确保RPD和TPD的值被正确设置
            if (data.rpd !== undefined) {
                document.getElementById('rpd-value').textContent = data.rpd;
            } else {
                document.getElementById('rpd-value').textContent = "0";
            }
            
            if (data.tpd !== undefined) {
                document.getElementById('tpd-value').textContent = data.tpd;
            } else {
                document.getElementById('tpd-value').textContent = "0";
            }
            
            // 更新每个API密钥的RPM和TPM值
            if (data.key_stats && data.key_stats.length > 0) {
                data.key_stats.forEach(keyStat => {
                    // 查找对应的密钥元素
                    const keyElements = document.querySelectorAll(`.key-item[data-key="${keyStat.key}"]`);
                    keyElements.forEach(keyElement => {
                        // 更新密钥的RPM和TPM
                        const rpmElement = keyElement.querySelector('.rpm-value');
                        const tpmElement = keyElement.querySelector('.tpm-value');
                        
                        if (rpmElement) {
                            rpmElement.textContent = keyStat.rpm || 0;
                        }
                        
                        if (tpmElement) {
                            tpmElement.textContent = keyStat.tpm || 0;
                        }
                        
                        // 更新得分
                        const scoreElement = keyElement.querySelector('.key-score');
                        if (scoreElement && keyStat.score !== undefined) {
                            scoreElement.textContent = `${keyStat.score.toFixed(2)}`;
                        }
                        
                        // 更新调用次数和成功率
                        const totalCallsElement = keyElement.querySelector('.key-stat:nth-child(1)');
                        const successRateElement = keyElement.querySelector('.key-stat:nth-child(2)');
                        
                        if (totalCallsElement) {
                            totalCallsElement.textContent = `调用: ${keyStat.total_calls}`;
                        }
                        
                        if (successRateElement) {
                            const successRatePercent = (keyStat.success_rate || 0) * 100;
                            successRateElement.textContent = `成功率: ${successRatePercent.toFixed(1)}%`;
                        }
                    });
                });
            }
            
            // 更新最后更新时间
            const now = new Date();
            const timeStr = now.toLocaleTimeString();
            document.getElementById('dashboard-last-update').textContent = `上次更新: ${timeStr}`;
        })
        .catch(error => {
            console.error('获取请求统计数据失败:', error);
        });
}

// 启动自动更新
function startAutoUpdate() {
    console.log(`启动自动更新，间隔：${AUTO_UPDATE_INTERVAL}秒`);
    
    // 清除现有的定时器
    if (autoUpdateTimer) {
        clearInterval(autoUpdateTimer);
    }
    if (statsUpdateTimer) {
        clearInterval(statsUpdateTimer);
    }
    if (rateUpdateTimer) {
        clearInterval(rateUpdateTimer);
    }
    
    // 设置速率监控更新定时器
    rateUpdateTimer = setInterval(() => {
        console.log(`速率监控更新 (${RATE_REFRESH_INTERVAL}秒)`);
        loadCurrentRequestStats();
    }, RATE_REFRESH_INTERVAL * 1000);
    
    // 设置统计信息更新定时器
    statsUpdateTimer = setInterval(() => {
        console.log(`统计信息更新 (${STATS_REFRESH_INTERVAL}秒)`);
        loadStats();
    }, STATS_REFRESH_INTERVAL * 1000);
    
    // 设置API密钥状态更新定时器
    autoUpdateTimer = setInterval(() => {
        console.log(`API密钥状态更新 (${AUTO_UPDATE_INTERVAL}秒)`);
        
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
                
                // 更新当前使用的API密钥信息
                loadCurrentKeyInfo();
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
    console.log("页面加载完成，开始初始化...");
    
    // 初始化Toast容器
    if (!document.getElementById('toast-container')) {
        const toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.className = 'toast-container';
        document.body.appendChild(toastContainer);
    }
    
    // 加载初始数据
    console.log("加载初始数据...");
    loadKeys();
    loadStats();
    loadCurrentRequestStats();
    
    // 初始化排序按钮
    initSortButtons();
    
    // 启动自动更新
    console.log("启动自动更新...");
    startAutoUpdate();
    
    // 更新API地址显示
    updateApiEndpoints();
    
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
        
        addKey(key, balance);
    });
    
    // 添加批量密钥表单提交事件
    document.getElementById('batch-add-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const keysInput = document.getElementById('batch-keys').value;
        const balance = document.getElementById('batch-balance').value;
        
        const keys = parseKeys(keysInput);
        
        if (keys.length === 0) {
            showToast('请输入至少一个有效的 API 密钥', 'error');
            return;
        }
        
        batchAddKeys(keys, balance);
    });
    
    // 添加刷新按钮事件
    document.getElementById('refresh-keys').addEventListener('click', function() {
        refreshAllKeysBalance(false); // 明确使用非静默模式
    });
    
    // 添加查看日志按钮事件
    document.getElementById('view-logs').addEventListener('click', function() {
        showLogViewer();
    });
    
    // 添加关闭日志查看器事件
    document.getElementById('log-close').addEventListener('click', function() {
        hideLogViewer();
    });
    
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
        console.log('轮询选中密钥按钮点击，选中的密钥:', selectedKeys);
        
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
    console.log("对所有密钥排序，字段:", field, "方向:", direction);
    
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
            console.log("排序按钮点击:", this.dataset.sort);
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
        sortKeys(unselectedKeyElements, currentSortField, currentSortDirection) : 
        unselectedKeyElements;
    
    // 合并选中和未选中的密钥
    const allSortedKeys = [...selectedKeyElements, ...sortedUnselectedKeys];
    
    // 清空容器并重新添加排序后的元素
    container.innerHTML = '';
    allSortedKeys.forEach(el => container.appendChild(el));
    
    // 重新绑定事件监听器
    bindKeyEvents();
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