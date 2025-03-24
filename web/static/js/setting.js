/**
 @author: AI
 @since: 2025/3/23 22:30:16
 @desc:
 **/

// 计时器相关常量
const AUTO_UPDATE_INTERVAL = 'auto_update_interval'; // 自动更新间隔
const STATS_REFRESH_INTERVAL = 'stats_refresh_interval'; // 统计刷新间隔
const RATE_REFRESH_INTERVAL = 'rate_refresh_interval'; // 速率刷新间隔
const RETRY_DELAY_MS = 'retry_delay_ms'; // 重试延迟毫秒
const RECOVERY_INTERVAL = 'recovery_interval'; // 恢复间隔
const TOAST_DISPLAY_TIME = 1500; // Toast显示时间（毫秒）

// 保存原始的模型列表数据
let allModelsList = [];

document.addEventListener('DOMContentLoaded', function() {
    // 加载配置
    loadSettings();
    
    // 加载模型列表
    loadModelList();

    // 绑定保存按钮点击事件
    document.getElementById('save-settings').addEventListener('click', function() {
        saveSettings();
    });

    // 重启程序按钮点击事件
    document.getElementById('restart-app').addEventListener('click', function() {
        // 先保存设置，然后重启程序
        saveSettings(function() {
            // 保存成功后，调用重启API
            fetch('/system/restart', {
                method: 'POST'
            })
            .then(response => response.json())
            .then(data => {
                showToast(data.message || '程序重启中，请稍候...', 'info');
                // 稍等后关闭页面（因为重启后页面会自动刷新）
                setTimeout(() => {
                    window.close();
                }, 2000);
            })
            .catch(error => {
                console.error('重启程序失败:', error);
                showToast('重启程序失败: ' + error, 'error');
            });
        });
    });

    // 重新加载按钮点击事件
    document.getElementById('reload-settings').addEventListener('click', function() {
        // 先保存设置，然后在保存完成后重新加载
        try {
            // 显示正在保存的消息
            showToast('正在保存并重新加载配置...', 'info');
            
            // 收集表单数据
            const config = {
                server: {
                    port: getValue('server-port')
                },
                api_proxy: {
                    base_url: getValue('api-base-url'),
                    model_key_strategies: modelKeyStrategies.hasOwnProperty ? modelKeyStrategies : {}, // 确保是对象
                    retry: {
                        max_retries: getValue('max-retries'),
                        [RETRY_DELAY_MS]: getValue('retry-delay'),
                        retry_on_status_codes: getValue('retry-status-codes')
                            .split(',')
                            .map(code => parseInt(code.trim()))
                            .filter(code => !isNaN(code)),
                        retry_on_network_errors: getValue('retry-network-errors')
                    }
                },
                proxy: {
                    enabled: getValue('proxy-enabled'),
                    proxy_type: getValue('proxy-type'),
                    http_proxy: getValue('http-proxy'),
                    https_proxy: getValue('https-proxy'),
                    socks_proxy: getValue('socks-proxy')
                },
                app: {
                    title: getValue('app-title'),
                    min_balance_threshold: getValue('min-balance'),
                    max_balance_display: getValue('max-balance'),
                    items_per_page: getValue('items-per-page'),
                    max_stats_entries: getValue('max-stats'),
                    [RECOVERY_INTERVAL]: getValue('recovery-interval'),
                    max_consecutive_failures: getValue('max-failures'),
                    hide_icon: getValue('hide-icon'),
                    balance_weight: getValue('balance-weight'),
                    success_rate_weight: getValue('success-rate-weight'),
                    rpm_weight: getValue('rpm-weight'),
                    tpm_weight: getValue('tpm-weight'),
                    [AUTO_UPDATE_INTERVAL]: getValue('auto-update'),
                    [STATS_REFRESH_INTERVAL]: getValue('stats-refresh'),
                    [RATE_REFRESH_INTERVAL]: getValue('rate-refresh')
                },
                log: {
                    max_size_mb: getValue('log-max-size'),
                    level: getValue('log-level')
                }
            };

            // 发送到服务器
            fetch('/settings/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(config)
            })
            .then(response => {
                if (!response.ok) {
                    throw new Error('保存失败，状态码: ' + response.status);
                }
                return response.json();
            })
            .then(data => {
                console.log('保存成功，准备重新加载');
                
                // 通知主页刷新配置和重启监控
                try {
                    if (window.opener && !window.opener.closed) {
                        if (typeof window.opener.startAutoUpdate === 'function') {
                            window.opener.startAutoUpdate();
                        }
                    }
                } catch (e) {
                    console.error('尝试通知主页时出错:', e);
                }
                
                // 强制刷新页面配置
                loadSettings();
                showToast('配置已保存并重新加载', 'success');
            })
            .catch(error => {
                console.error('保存失败:', error);
                showToast('保存失败: ' + error.message, 'error');
            });
        } catch (err) {
            console.error('处理过程中发生异常:', err);
            showToast('发生异常: ' + err.message, 'error');
        }
    });
    
    // 绑定返回主页按钮事件
    document.getElementById('back-to-home').addEventListener('click', function() {
        // 先保存设置，然后在保存完成后返回主页
        try {
            // 显示正在保存的消息
            showToast('正在保存配置，即将返回主页...', 'info');
            
            // 收集表单数据
            const config = {
                server: {
                    port: getValue('server-port')
                },
                api_proxy: {
                    base_url: getValue('api-base-url'),
                    model_key_strategies: modelKeyStrategies.hasOwnProperty ? modelKeyStrategies : {}, // 确保是对象
                    retry: {
                        max_retries: getValue('max-retries'),
                        [RETRY_DELAY_MS]: getValue('retry-delay'),
                        retry_on_status_codes: getValue('retry-status-codes')
                            .split(',')
                            .map(code => parseInt(code.trim()))
                            .filter(code => !isNaN(code)),
                        retry_on_network_errors: getValue('retry-network-errors')
                    }
                },
                proxy: {
                    enabled: getValue('proxy-enabled'),
                    proxy_type: getValue('proxy-type'),
                    http_proxy: getValue('http-proxy'),
                    https_proxy: getValue('https-proxy'),
                    socks_proxy: getValue('socks-proxy')
                },
                app: {
                    title: getValue('app-title'),
                    min_balance_threshold: getValue('min-balance'),
                    max_balance_display: getValue('max-balance'),
                    items_per_page: getValue('items-per-page'),
                    max_stats_entries: getValue('max-stats'),
                    [RECOVERY_INTERVAL]: getValue('recovery-interval'),
                    max_consecutive_failures: getValue('max-failures'),
                    hide_icon: getValue('hide-icon'),
                    balance_weight: getValue('balance-weight'),
                    success_rate_weight: getValue('success-rate-weight'),
                    rpm_weight: getValue('rpm-weight'),
                    tpm_weight: getValue('tpm-weight'),
                    [AUTO_UPDATE_INTERVAL]: getValue('auto-update'),
                    [STATS_REFRESH_INTERVAL]: getValue('stats-refresh'),
                    [RATE_REFRESH_INTERVAL]: getValue('rate-refresh')
                },
                log: {
                    max_size_mb: getValue('log-max-size'),
                    level: getValue('log-level')
                }
            };

            // 发送到服务器
            fetch('/settings/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(config)
            })
            .then(response => {
                if (!response.ok) {
                    throw new Error('保存失败，状态码: ' + response.status);
                }
                return response.json();
            })
            .then(data => {
                showToast('配置已保存，正在返回主页...', 'success');
                
                // 短暂延迟后返回主页
                setTimeout(function() {
                    window.location.href = '/';
                }, 800);
            })
            .catch(error => {
                console.error('保存失败:', error);
                showToast('保存失败: ' + error.message + '，3秒后仍将返回主页', 'error');
                
                // 即使发生错误，也在一定时间后返回主页
                setTimeout(function() {
                    window.location.href = '/';
                }, 3000);
            });
        } catch (err) {
            console.error('处理过程中发生异常:', err);
            showToast('发生异常: ' + err.message + '，3秒后仍将返回主页', 'error');
            
            // 即使发生异常，也在一定时间后返回主页
            setTimeout(function() {
                window.location.href = '/';
            }, 3000);
        }
    });
    
    // 绑定添加模型策略按钮点击事件
    document.getElementById('add-model-strategy').addEventListener('click', function() {
        addModelStrategy();
    });
    
    // 添加Ctrl+S快捷键支持
    document.addEventListener('keydown', function(e) {
        if ((e.ctrlKey || e.metaKey) && e.key === 's') {
            e.preventDefault(); // 阻止浏览器默认保存页面行为
            saveSettings();
            showToast('已使用快捷键保存设置 (Ctrl+S)', 'success');
        }
    });
});

/**
 * 加载模型列表
 */
function loadModelList() {
    const modelSelect = document.getElementById('new-model-name');
    const baseUrl = window.location.origin;
    
    // 创建搜索输入框和下拉列表容器
    convertToSearchableDropdown();
    
    // 设置加载中的状态
    modelSelect.innerHTML = '<option value="" disabled selected>正在加载模型列表...</option>';
    
    fetchModelList();
    
    // 内部函数，用于获取模型列表
    function fetchModelList() {
        // 获取API基础URL
        
        fetch(`${baseUrl}/v1/models`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        })
            .then(response => {
                if (!response.ok) {
                    throw new Error('网络响应不正常，状态码: ' + response.status);
                }
                return response.json();
            })
            .then(data => {
                // 清空下拉列表
                modelSelect.innerHTML = '';
                
                // 检查是否有可用的模型（适配新的数据格式）
                if (data && data.data && Array.isArray(data.data) && data.data.length > 0) {
                    // 提取模型ID列表
                    const modelIds = data.data.map(model => model.id);
                    
                    // 保存所有模型ID到全局变量
                    allModelsList = modelIds;
                    
                    // 添加提示选项
                    const defaultOption = document.createElement('option');
                    defaultOption.value = '';
                    defaultOption.text = '请选择模型...';
                    defaultOption.disabled = true;
                    defaultOption.selected = true;
                    modelSelect.appendChild(defaultOption);
                    
                    // 添加模型选项
                    modelIds.forEach(model => {
                        const option = document.createElement('option');
                        option.value = model;
                        option.text = model;
                        modelSelect.appendChild(option);
                    });
                    
                    // 如果已经转换为可搜索下拉框，更新下拉框选项
                    const searchInput = document.getElementById('model-search-input');
                    if (searchInput) {
                        if (typeof updateDropdownOptionsFunction === 'function') {
                            updateDropdownOptionsFunction('');
                        }
                    }
                } else {
                    // 如果没有模型，尝试重新获取
                    console.log('没有获取到模型，准备尝试手动重新获取');
                    
                    // 显示正在重试的选项
                    const retryOption = document.createElement('option');
                    retryOption.value = '';
                    retryOption.text = '正在重新获取模型列表...';
                    retryOption.disabled = true;
                    modelSelect.appendChild(retryOption);
                    
                    // 尝试先刷新可用模型，再重新获取
                    fetch('/keys/refresh')
                        .then(response => {
                            if (!response.ok) {
                                throw new Error('刷新模型列表失败');
                            }
                            return response.json();
                        })
                        .then(refreshData => {
                            // 等待一秒后重新获取模型列表
                            setTimeout(() => {
                                fetchModelList();
                            }, 1000);
                        })
                        .catch(error => {
                            console.error('刷新模型列表失败:', error);
                            
                            // 显示错误信息，并提供建议
                            const errorMessage = `获取模型列表失败: ${error.message}`;
                            console.error(errorMessage);
                            showNoModelsOptions(`无法从 ${baseUrl}/v1/models 获取模型列表，请检查API基础URL和网络连接`);
                        });
                }
            })
    }
    
    // 显示无模型选项并提供手动输入功能
    function showNoModelsOptions(message) {
        // 清空下拉列表
        modelSelect.innerHTML = '';
        
        // 添加提示选项
        const noModelOption = document.createElement('option');
        noModelOption.value = '';
        noModelOption.text = message;
        noModelOption.disabled = true;
        modelSelect.appendChild(noModelOption);
        
        // 添加"切换到手动输入"选项
        const manualOption = document.createElement('option');
        manualOption.value = 'manual_input';
        manualOption.text = '点击此处切换到手动输入...';
        modelSelect.appendChild(manualOption);
        
        // 监听选择事件
        modelSelect.addEventListener('change', function(e) {
            if (e.target.value === 'manual_input') {
                // 将下拉列表转换为文本输入框
                convertToTextInput();
            }
        });
        
        // 显示提示
        showToast(message, 'info');
    }
    
    // 将下拉列表转换为文本输入框
    function convertToTextInput() {
        const modelInput = document.createElement('input');
        modelInput.type = 'text';
        modelInput.className = 'form-control';
        modelInput.id = 'new-model-name';
        modelInput.placeholder = '模型名称 (例如: openai/gpt-4)';
        
        // 替换下拉列表
        modelSelect.parentNode.replaceChild(modelInput, modelSelect);
        
        // 聚焦到输入框
        modelInput.focus();
    }

    // 将标准下拉列表转换为可搜索的下拉列表
    function convertToSearchableDropdown() {
        // 创建一个包含搜索框和下拉列表的容器
        const container = document.createElement('div');
        container.className = 'searchable-dropdown';
        container.style.position = 'relative';
        
        // 创建搜索输入框
        const searchInput = document.createElement('input');
        searchInput.type = 'text';
        searchInput.className = 'form-control';
        searchInput.id = 'model-search-input';
        searchInput.placeholder = '搜索或输入模型名称...';
        
        // 创建下拉选项容器
        const dropdownContainer = document.createElement('div');
        dropdownContainer.className = 'dropdown-options';
        dropdownContainer.style.position = 'absolute';
        dropdownContainer.style.width = '100%';
        dropdownContainer.style.maxHeight = '300px';
        dropdownContainer.style.overflowY = 'auto';
        dropdownContainer.style.zIndex = '1000';
        dropdownContainer.style.backgroundColor = '#fff';
        dropdownContainer.style.border = '1px solid #ced4da';
        dropdownContainer.style.borderRadius = '0.25rem';
        dropdownContainer.style.display = 'none';
        
        // 隐藏原始下拉列表
        modelSelect.style.display = 'none';
        
        // 在原始下拉列表的位置插入新元素
        modelSelect.parentNode.insertBefore(container, modelSelect);
        container.appendChild(searchInput);
        container.appendChild(dropdownContainer);
        
        // 搜索输入框事件监听
        searchInput.addEventListener('input', function() {
            const searchText = this.value.toLowerCase();
            updateDropdownOptions(searchText);
            dropdownContainer.style.display = 'block';
        });
        
        // 点击输入框显示所有选项
        searchInput.addEventListener('click', function() {
            updateDropdownOptions(this.value.toLowerCase());
            dropdownContainer.style.display = 'block';
        });
        
        // 全局定义updateDropdownOptions函数，这样在任何位置都可以访问
        let updateDropdownOptionsFunction = function() {};
        
        // 在文档其他地方点击时隐藏下拉框
        document.addEventListener('click', function(e) {
            if (e.target !== searchInput && !dropdownContainer.contains(e.target)) {
                dropdownContainer.style.display = 'none';
            }
        });
        
        // 更新下拉框选项的函数
        function updateDropdownOptions(searchText) {
            dropdownContainer.innerHTML = '';
            
            // 如果还没有获取到模型列表，显示加载中
            if (allModelsList.length === 0) {
                const loadingItem = document.createElement('div');
                loadingItem.className = 'dropdown-item';
                loadingItem.textContent = '正在加载模型列表...';
                dropdownContainer.appendChild(loadingItem);
                return;
            }
            
            const filteredModels = allModelsList.filter(model => 
                model.toLowerCase().includes(searchText.toLowerCase())
            );
            
            if (filteredModels.length === 0) {
                const noMatchItem = document.createElement('div');
                noMatchItem.className = 'dropdown-item';
                noMatchItem.textContent = '没有匹配的模型';
                
                if (searchText.trim() !== '') {
                    // 添加"添加自定义模型"选项
                    noMatchItem.textContent = '没有匹配的模型，点击添加自定义';
                    noMatchItem.style.color = '#0d6efd';
                    noMatchItem.style.cursor = 'pointer';
                    noMatchItem.addEventListener('click', function() {
                        selectModel(searchText);
                    });
                }
                
                dropdownContainer.appendChild(noMatchItem);
            } else {
                filteredModels.forEach(model => {
                    const item = document.createElement('div');
                    item.className = 'dropdown-item';
                    item.textContent = model;
                    item.style.cursor = 'pointer';
                    item.addEventListener('click', function() {
                        selectModel(model);
                    });
                    dropdownContainer.appendChild(item);
                });
            }
        }
        
        // 将updateDropdownOptions函数赋值给全局变量
        updateDropdownOptionsFunction = updateDropdownOptions;
        
        // 选择模型的函数
        function selectModel(model) {
            searchInput.value = model;
            dropdownContainer.style.display = 'none';
            
            // 设置原始select元素的值
            const option = Array.from(modelSelect.options).find(opt => opt.value === model);
            if (option) {
                modelSelect.value = model;
            } else {
                // 如果不存在该选项，创建一个新选项
                const newOption = document.createElement('option');
                newOption.value = model;
                newOption.text = model;
                modelSelect.appendChild(newOption);
                modelSelect.value = model;
            }
            
            // 触发change事件
            const event = new Event('change', { bubbles: true });
            modelSelect.dispatchEvent(event);
        }
    }
}

// 全局变量，存储模型策略
let modelKeyStrategies = {};

/**
 * 加载设置
 */
function loadSettings() {
    fetch('/settings/config')
        .then(response => {
            if (!response.ok) {
                throw new Error('网络响应不正常');
            }
            return response.json();
        })
        .then(data => {
            populateForm(data);
            showToast('配置加载成功', 'success');
        })
        .catch(error => {
            console.error('获取配置失败:', error);
            showToast('获取配置失败: ' + error.message, 'error');
        });
}

/**
 * 填充表单数据
 * @param {Object} config - 配置对象
 */
function populateForm(config) {
    // 清空表单
    document.getElementById('settings-form').reset();

    // 服务器设置
    setValue('server-port', config.server.port);

    // API代理设置
    setValue('api-base-url', config.api_proxy.base_url);
    
    // 如果存在模型特定策略，则填充
    if (config.api_proxy.model_key_strategies) {
        modelKeyStrategies = config.api_proxy.model_key_strategies;
        console.log('从API代理加载模型策略:', modelKeyStrategies);
        updateModelStrategiesTable();
    } else {
        console.log('从API代理没有找到模型策略，尝试从APP读取');
        // 兼容旧版本的配置结构
        if (config.app && config.app.model_key_strategies) {
            modelKeyStrategies = config.app.model_key_strategies;
            console.log('从APP加载模型策略:', modelKeyStrategies);
        } else {
            console.log('没有找到任何模型策略配置');
            modelKeyStrategies = {};
        }
        updateModelStrategiesTable();
    }
    
    // 重试配置
    setValue('max-retries', config.api_proxy.retry.max_retries);
    setValue('retry-delay', config.api_proxy.retry[RETRY_DELAY_MS]);
    setValue('retry-status-codes', config.api_proxy.retry.retry_on_status_codes.join(','));
    setValue('retry-network-errors', config.api_proxy.retry.retry_on_network_errors);
    
    // 代理设置
    setValue('proxy-enabled', config.proxy.enabled);
    setValue('proxy-type', config.proxy.proxy_type);
    setValue('http-proxy', config.proxy.http_proxy);
    setValue('https-proxy', config.proxy.https_proxy);
    setValue('socks-proxy', config.proxy.socks_proxy);
    
    // 应用设置
    setValue('app-title', config.app.title);
    setValue('min-balance', config.app.min_balance_threshold);
    setValue('max-balance', config.app.max_balance_display);
    setValue('items-per-page', config.app.items_per_page);
    setValue('max-stats', config.app.max_stats_entries);
    setValue('recovery-interval', config.app[RECOVERY_INTERVAL]);
    setValue('max-failures', config.app.max_consecutive_failures);
    setValue('hide-icon', config.app.hide_icon);
    
    // 权重配置
    setValue('balance-weight', config.app.balance_weight);
    setValue('success-rate-weight', config.app.success_rate_weight);
    setValue('rpm-weight', config.app.rpm_weight);
    setValue('tpm-weight', config.app.tpm_weight);
    
    // 自动更新配置
    setValue('auto-update', config.app[AUTO_UPDATE_INTERVAL]);
    setValue('stats-refresh', config.app[STATS_REFRESH_INTERVAL]);
    setValue('rate-refresh', config.app[RATE_REFRESH_INTERVAL]);
    
    // 日志设置
    setValue('log-max-size', config.log.max_size_mb);
    setValue('log-level', config.log.level || 'warn'); // 设置日志等级，默认为warn
}

/**
 * 更新模型策略表格
 */
function updateModelStrategiesTable() {
    const tableBody = document.getElementById('model-strategies-body');
    
    // 清空表格内容
    tableBody.innerHTML = '';
    
    // 如果没有策略，显示空消息
    if (Object.keys(modelKeyStrategies).length === 0) {
        tableBody.innerHTML = '<tr class="text-center text-muted"><td colspan="3">暂无特定模型策略配置</td></tr>';
        return;
    }
    
    // 填充表格
    for (const modelName in modelKeyStrategies) {
        const strategy = modelKeyStrategies[modelName];
        const strategyText = getStrategyText(strategy);
        
        const row = document.createElement('tr');
        // 添加数据属性
        row.setAttribute('data-model-name', modelName);
        row.setAttribute('data-strategy-id', strategy);
        
        row.innerHTML = `
            <td title="${modelName}" data-model-name="${modelName}">${modelName}</td>
            <td data-strategy-id="${strategy}">${strategyText}</td>
            <td>
                <div class="d-flex justify-content-end" style="gap: 4px;">
                    <button type="button" class="btn btn-sm btn-secondary" onclick="editModelStrategy('${modelName}', ${strategy})">
                        <i class="bi bi-pencil-square"></i> 修改
                    </button>
                    <button type="button" class="btn btn-sm btn-danger" onclick="removeModelStrategy('${modelName}')">
                        <i class="bi bi-trash"></i> 删除
                    </button>
                </div>
            </td>
        `;
        
        tableBody.appendChild(row);
    }
}

/**
 * 获取策略文本描述
 * @param {number} strategy - 策略ID
 * @returns {string} 策略描述
 */
function getStrategyText(strategy) {
    switch (parseInt(strategy)) {
        case 1: return '策略1 - 高成功率';
        case 2: return '策略2 - 高分数';
        case 3: return '策略3 - 低RPM';
        case 4: return '策略4 - 低TPM';
        case 5: return '策略5 - 高余额';
        case 6: return '策略6 - 普通（默认）';
        default: return `未知策略(${strategy})`;
    }
}

/**
 * 编辑模型策略
 * @param {string} modelName - 模型名称
 * @param {number} strategy - 当前策略ID
 */
function editModelStrategy(modelName, strategy) {
    const modelNameElement = document.getElementById('new-model-name');
    const strategySelect = document.getElementById('new-model-strategy');
    const addButton = document.getElementById('add-model-strategy');
    
    // 处理搜索框情况
    const searchInput = document.getElementById('model-search-input');
    if (searchInput) {
        // 如果使用的是搜索框，则设置搜索框值
        searchInput.value = modelName;
        
        // 更新下拉选项并选中对应的选项
        if (allModelsList.includes(modelName)) {
            // 如果在模型列表中找到了
            const option = Array.from(modelNameElement.options).find(opt => opt.value === modelName);
            if (option) {
                modelNameElement.value = modelName;
            } else {
                // 如果找不到对应的选项，可能是因为搜索框刚创建，需要填充选项
                const newOption = document.createElement('option');
                newOption.value = modelName;
                newOption.text = modelName;
                modelNameElement.appendChild(newOption);
                modelNameElement.value = modelName;
            }
        } else {
            // 如果模型不在列表中，可能是旧数据，禁止编辑
            showToast(`模型 "${modelName}" 不在当前模型列表中，无法编辑`, 'error');
            return;
        }
    } else {
        // 根据元素类型设置模型名称
        if (modelNameElement.tagName.toLowerCase() === 'select') {
            // 查找对应的选项
            const optionExists = Array.from(modelNameElement.options).some(option => {
                if (option.value === modelName) {
                    modelNameElement.value = modelName;
                    return true;
                }
                return false;
            });
            
            // 如果下拉列表中没有这个选项，显示错误
            if (!optionExists) {
                showToast(`模型 "${modelName}" 不在当前模型列表中，无法编辑`, 'error');
                return;
            }
        } else {
            // 如果是文本框，显示错误
            showToast('当前模式不支持修改模型，请返回主页后重新打开设置页面', 'error');
            return;
        }
    }
    
    // 选中当前策略
    strategySelect.value = strategy;
    
    // 提示用户
    showToast(`请编辑 "${modelName}" 的策略设置，然后点击"保存修改"按钮`, 'info');
    
    // 给搜索框添加只读属性防止修改模型名
    if (searchInput) {
        searchInput.readOnly = true;
        searchInput.style.backgroundColor = '#f8f9fa';
        searchInput.style.cursor = 'not-allowed';
    }
    
    // 修改添加按钮文本为"保存修改"并添加样式
    addButton.textContent = '保存修改';
    addButton.classList.add('btn-save-modify');
    
    // 保存旧模型名称，用于在保存时删除旧记录
    addButton.setAttribute('data-editing', modelName);
}

/**
 * 添加模型策略
 */
function addModelStrategy() {
    const modelNameElement = document.getElementById('new-model-name');
    const strategySelect = document.getElementById('new-model-strategy');
    const addButton = document.getElementById('add-model-strategy');
    const searchInput = document.getElementById('model-search-input');
    
    // 获取模型名称，兼容下拉列表和文本输入框两种情况
    let modelName = '';
    if (searchInput) {
        // 如果使用的是搜索框
        modelName = searchInput.value.trim();
    } else if (modelNameElement.tagName.toLowerCase() === 'select') {
        // 如果是下拉列表
        modelName = modelNameElement.value.trim();
    } else {
        // 如果是文本框
        modelName = modelNameElement.value.trim();
    }
    
    const strategy = strategySelect.value;
    
    // 验证模型名称不能为空
    if (!modelName) {
        showToast('请选择或输入模型名称', 'error');
        return;
    }
    
    // 检查是否在编辑模式
    const editingModelName = addButton.getAttribute('data-editing');
    
    // 如果不是编辑模式，验证模型是否在列表中
    if (!editingModelName && !allModelsList.includes(modelName)) {
        showToast(`模型 "${modelName}" 不在当前模型列表中，无法添加`, 'error');
        return;
    }
    
    if (editingModelName) {
        // 如果模型名称已更改，则删除旧记录
        if (editingModelName !== modelName) {
            delete modelKeyStrategies[editingModelName];
        }
        
        // 清除编辑状态
        addButton.removeAttribute('data-editing');
        addButton.textContent = '添加';
        addButton.classList.remove('btn-save-modify');
        
        // 解除搜索框的只读状态
        if (searchInput) {
            searchInput.readOnly = false;
            searchInput.style.backgroundColor = '';
            searchInput.style.cursor = '';
        }
    }
    
    // 添加到模型策略对象
    modelKeyStrategies[modelName] = parseInt(strategy);
    
    // 更新表格
    updateModelStrategiesTable();
    
    // 重置输入
    if (searchInput) {
        searchInput.value = '';
        // 聚焦到搜索框
        searchInput.focus();
    } else if (modelNameElement.tagName.toLowerCase() === 'select') {
        modelNameElement.selectedIndex = 0;
    } else {
        // 如果是文本框，清空输入
        modelNameElement.value = '';
    }
    
    // 重置策略选择
    strategySelect.selectedIndex = 0;
    
    // 显示成功消息
    showToast(`已${editingModelName ? '更新' : '添加'} ${modelName} 的策略配置`, 'success');
}

/**
 * 移除模型策略
 * @param {string} modelName - 模型名称
 */
function removeModelStrategy(modelName) {
    // 从模型策略对象中删除
    delete modelKeyStrategies[modelName];
    
    // 更新表格
    updateModelStrategiesTable();
    
    // 显示成功消息
    showToast(`已移除 ${modelName} 的策略配置`, 'success');
}

/**
 * 设置表单元素的值
 * @param {string} id - 元素ID
 * @param {any} value - 要设置的值
 */
function setValue(id, value) {
    const element = document.getElementById(id);
    if (!element) {
        console.warn(`元素 ${id} 不存在`);
        return;
    }
    
    if (element.type === 'checkbox') {
        element.checked = Boolean(value);
    } else {
        element.value = value;
    }
}

/**
 * 获取表单元素的值
 * @param {string} id - 元素ID
 * @returns {any} 元素的值
 */
function getValue(id) {
    const element = document.getElementById(id);
    if (!element) {
        console.warn(`元素 ${id} 不存在`);
        return null;
    }
    
    if (element.type === 'checkbox') {
        return element.checked;
    } else if (element.type === 'number') {
        return element.value === '' ? 0 : Number(element.value);
    } else {
        return element.value;
    }
}

/**
 * 保存设置
 * @param {Function} callback - 可选的回调函数，在保存成功后执行
 */
function saveSettings(callback) {
    // 验证设置是否有效
    if (!validateSettings()) {
        showToast('设置验证失败，请检查输入', 'error');
        return;
    }

    // 显示保存中的消息
    showToast('正在保存设置...', 'info');

    // 获取当前应用标题
    let appTitle = getValue('app-title');
    
    // 如果应用标题已修改，确保保留版本号
    if (appTitle && !appTitle.includes('FlowSilicon v')) {
        // 尝试从旧标题中提取版本号
        let oldTitle = document.querySelector('.title-container h1')?.textContent || '';
        let versionMatch = oldTitle.match(/FlowSilicon\s+(v[\d\.]+)/i);
        
        if (versionMatch && versionMatch[1]) {
            // 保留版本号
            appTitle = `${appTitle} ${versionMatch[1]}`;
            // 更新输入框显示
            document.getElementById('app-title').value = appTitle;
        }
    }

    // 收集表单数据
    const config = {
        server: {
            port: getValue('server-port')
        },
        api_proxy: {
            base_url: getValue('api-base-url'),
            retry: {
                max_retries: getValue('max-retries'),
                retry_delay_ms: getValue('retry-delay'),
                retry_on_status_codes: parseStatusCodes(getValue('retry-status-codes')),
                retry_on_network_errors: getValue('retry-network-errors')
            }
        },
        proxy: {
            http_proxy: getValue('http-proxy'),
            https_proxy: getValue('https-proxy'),
            socks_proxy: getValue('socks-proxy'),
            proxy_type: getValue('proxy-type'),
            enabled: getValue('proxy-enabled')
        },
        app: {
            title: appTitle,
            min_balance_threshold: getValue('min-balance'),
            max_balance_display: getValue('max-balance'),
            items_per_page: getValue('items-per-page'),
            max_stats_entries: getValue('max-stats'),
            recovery_interval: getValue('recovery-interval'),
            max_consecutive_failures: getValue('max-failures'),
            model_key_strategies: collectModelStrategies(),
            hide_icon: getValue('hide-icon'),
            balance_weight: getValue('balance-weight'),
            success_rate_weight: getValue('success-rate-weight'),
            rpm_weight: getValue('rpm-weight'),
            tpm_weight: getValue('tpm-weight'),
            [AUTO_UPDATE_INTERVAL]: getValue('auto-update'),
            [STATS_REFRESH_INTERVAL]: getValue('stats-refresh'),
            [RATE_REFRESH_INTERVAL]: getValue('rate-refresh')
        },
        log: {
            max_size_mb: getValue('log-max-size'),
            level: getValue('log-level')
        }
    };

    // 发送到服务器
    fetch('/settings/config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(config)
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('保存失败，状态码: ' + response.status);
        }
        return response.json();
    })
    .then(data => {
        
        // 通知主页刷新配置和重启监控
        try {
            if (window.opener && !window.opener.closed) {
                if (typeof window.opener.startAutoUpdate === 'function') {
                    window.opener.startAutoUpdate();
                }
            }
        } catch (e) {
            console.error('通知主页失败:', e);
        }

        // 显示成功消息
        showToast('设置已保存', 'success');

        // 执行回调
        if (typeof callback === 'function') {
            callback();
        }
    })
    .catch(error => {
        console.error('保存设置失败:', error);
        showToast('保存设置失败: ' + error.message, 'error');
    });
}

/**
 * 保存设置并在成功后执行回调函数
 * @param {Function} callback - 保存成功后要执行的回调函数
 * @deprecated 请直接使用 saveSettings(callback) 函数
 */
function saveSettingsWithCallback(callback) {
    saveSettings(callback);
}

/**
 * 显示Toast通知
 * @param {string} message - 通知消息
 * @param {string} type - 通知类型（success/error/info）
 */
function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container');
    
    // 创建toast元素
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.setAttribute('role', 'alert');
    toast.setAttribute('aria-live', 'assertive');
    toast.setAttribute('aria-atomic', 'true');
    
    // 设置toast内容
    toast.innerHTML = `
        <div class="toast-header">
            <strong class="me-auto">${type === 'success' ? '成功' : type === 'error' ? '错误' : '信息'}</strong>
            <button type="button" class="btn-close" data-bs-dismiss="toast" aria-label="Close"></button>
        </div>
        <div class="toast-body">
            ${message}
        </div>
    `;
    
    // 添加到容器
    toastContainer.appendChild(toast);
    
    // 使用Bootstrap的Toast组件
    const bsToast = new bootstrap.Toast(toast, {
        autohide: true,
        delay: TOAST_DISPLAY_TIME
    });
    
    bsToast.show();
    
    // 监听关闭事件，移除DOM元素
    toast.addEventListener('hidden.bs.toast', function() {
        toast.remove();
    });
}

// 验证设置是否有效
function validateSettings() {
    // 检查服务器端口是否有效
    const port = getValue('server-port');
    if (isNaN(port) || port < 1 || port > 65535) {
        showToast('服务器端口必须是1-65535之间的有效数字', 'error');
        return false;
    }
    
    // 检查权重总和是否为1
    const balanceWeight = parseFloat(getValue('balance-weight')) || 0;
    const successRateWeight = parseFloat(getValue('success-rate-weight')) || 0;
    const rpmWeight = parseFloat(getValue('rpm-weight')) || 0;
    const tpmWeight = parseFloat(getValue('tpm-weight')) || 0;
    
    const totalWeight = balanceWeight + successRateWeight + rpmWeight + tpmWeight;
    if (Math.abs(totalWeight - 1) > 0.01) { // 允许0.01的误差
        showToast(`权重总和必须等于1，当前总和为${totalWeight.toFixed(2)}`, 'error');
        return false;
    }
    
    return true;
}

// 解析状态码字符串为数组
function parseStatusCodes(codesStr) {
    if (!codesStr) return [];
    return codesStr.split(',')
        .map(code => parseInt(code.trim()))
        .filter(code => !isNaN(code));
}

// 收集模型策略配置
function collectModelStrategies() {
    // 直接返回全局变量中的模型策略
    return modelKeyStrategies;
}
