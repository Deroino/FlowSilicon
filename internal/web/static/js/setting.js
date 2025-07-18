/**
 @author: AI
 @since: 2025/3/23 22:30:16
 @desc: 设置页面功能实现，包含配置加载、保存、模型策略管理和界面交互
 **/

// 计时器相关常量
const AUTO_UPDATE_INTERVAL = 'auto_update_interval'; // 自动更新间隔
const STATS_REFRESH_INTERVAL = 'stats_refresh_interval'; // 统计刷新间隔
const RATE_REFRESH_INTERVAL = 'rate_refresh_interval'; // 速率刷新间隔
const RETRY_DELAY_MS = 'retry_delay_ms'; // 重试延迟毫秒
const RECOVERY_INTERVAL = 'recovery_interval'; // 恢复间隔
const AUTO_DELETE_ZERO_BALANCE_KEYS = 'auto_delete_zero_balance_keys'; // 自动删除余额为0的密钥
const REFRESH_USED_KEYS_INTERVAL = 'refresh_used_keys_interval'; // 刷新已使用密钥余额的间隔
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

    // 绑定导出设置按钮点击事件
    document.getElementById('export-settings').addEventListener('click', function() {
        exportSettings();
    });

    // 绑定导入设置按钮点击事件
    document.getElementById('import-settings').addEventListener('click', function() {
        document.getElementById('settings-file-input').click();
    });

    // 监听文件输入变化事件
    document.getElementById('settings-file-input').addEventListener('change', function(event) {
        if (event.target.files.length > 0) {
            importSettings(event.target.files[0]);
        }
    });
    
    // 绑定生成API密钥按钮点击事件
    document.getElementById('generate-api-key').addEventListener('click', function() {
        const apiKey = generateApiKey();
        const apiKeyInput = document.getElementById('api-key');
        apiKeyInput.value = apiKey;
        
        // 复制到剪贴板（标记为生成的密钥）
        copyToClipboard(apiKey, true);
    });

    // 绑定复制API密钥按钮点击事件
    document.getElementById('copy-api-key').addEventListener('click', function() {
        const apiKeyInput = document.getElementById('api-key');
        const apiKey = apiKeyInput.value.trim();
        
        if (apiKey) {
            // 复制到剪贴板（标记为非生成的密钥）
            copyToClipboard(apiKey, false);
        } else {
            showToast('API密钥为空，无法复制', 'warning');
        }
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
                    [RATE_REFRESH_INTERVAL]: getValue('rate-refresh'),
                    [AUTO_DELETE_ZERO_BALANCE_KEYS]: getValue('auto-delete-zero-balance'),
                    [REFRESH_USED_KEYS_INTERVAL]: getValue('refresh-used-keys-interval')
                },
                log: {
                    max_size_mb: getValue('log-max-size'),
                    level: getValue('log-level')
                },
                request_settings: {
                    http_client: {
                        response_header_timeout: getValue('response-header-timeout'),
                        tls_handshake_timeout: getValue('tls-handshake-timeout'),
                        idle_conn_timeout: getValue('idle-conn-timeout'),
                        expect_continue_timeout: getValue('expect-continue-timeout'),
                        max_idle_conns: getValue('max-idle-conns'),
                        max_idle_conns_per_host: getValue('max-idle-conns-per-host'),
                        keep_alive: getValue('keep-alive'),
                        connect_timeout: getValue('connect-timeout')
                    },
                    proxy_handler: {
                        inference_timeout: getValue('inference-timeout'),
                        standard_timeout: getValue('standard-timeout'),
                        stream_timeout: getValue('stream-timeout'),
                        heartbeat_interval: getValue('heartbeat-interval'),
                        progress_interval: getValue('progress-interval'),
                        buffer_threshold: getValue('buffer-threshold'),
                        max_flush_interval: getValue('max-flush-interval'),
                        max_concurrency: getValue('max-concurrency'),
                        use_fake_streaming: getCheckbox('use-fake-streaming')
                    },
                    database: {
                        conn_max_lifetime: getValue('conn-max-lifetime'),
                        max_idle_conns: getValue('db-max-idle-conns')
                    },
                    defaults: {
                        max_tokens: getValue('default-max-tokens'),
                        image_size: getValue('default-image-size'),
                        max_chunks_per_doc: getValue('max-chunks-per-doc')
                    }
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
            // 收集表单数据
            const config = {
                server: {
                    port: getValue('server-port')
                },
                security:{
                    password_enabled: getValue('password-enabled'),
                    expiration_minutes: getValue('expiration-minutes'),
                    api_key_enabled: getValue('api-key-enabled'),
                    api_key: getValue('api-key'),
                    password: getValue('password')
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
                    [RATE_REFRESH_INTERVAL]: getValue('rate-refresh'),
                    [AUTO_DELETE_ZERO_BALANCE_KEYS]: getValue('auto-delete-zero-balance'),
                    [REFRESH_USED_KEYS_INTERVAL]: getValue('refresh-used-keys-interval')
                },
                log: {
                    max_size_mb: getValue('log-max-size'),
                    level: getValue('log-level')
                },
                request_settings: {
                    http_client: {
                        response_header_timeout: getValue('response-header-timeout'),
                        tls_handshake_timeout: getValue('tls-handshake-timeout'),
                        idle_conn_timeout: getValue('idle-conn-timeout'),
                        expect_continue_timeout: getValue('expect-continue-timeout'),
                        max_idle_conns: getValue('max-idle-conns'),
                        max_idle_conns_per_host: getValue('max-idle-conns-per-host'),
                        keep_alive: getValue('keep-alive'),
                        connect_timeout: getValue('connect-timeout')
                    },
                    proxy_handler: {
                        inference_timeout: getValue('inference-timeout'),
                        standard_timeout: getValue('standard-timeout'),
                        stream_timeout: getValue('stream-timeout'),
                        heartbeat_interval: getValue('heartbeat-interval'),
                        progress_interval: getValue('progress-interval'),
                        buffer_threshold: getValue('buffer-threshold'),
                        max_flush_interval: getValue('max-flush-interval'),
                        max_concurrency: getValue('max-concurrency'),
                        use_fake_streaming: getCheckbox('use-fake-streaming')
                    },
                    database: {
                        conn_max_lifetime: getValue('conn-max-lifetime'),
                        max_idle_conns: getValue('db-max-idle-conns')
                    },
                    defaults: {
                        max_tokens: getValue('default-max-tokens'),
                        image_size: getValue('default-image-size'),
                        max_chunks_per_doc: getValue('max-chunks-per-doc')
                    }
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
                    return response.json().then(errorData => {
                        showToast('保存设置失败: ' + errorData.error,'error');
                        throw new Error(errorData.error || `保存失败：${errorData.statusText}`);
                    });
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
                showToast('保存设置失败: ' + error.message, 'error');
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
        }
    });

    // 保存所有复选框的初始状态
    saveAllCheckboxStates();
});

/**
 * 加载模型列表
 */
function loadModelList() {
    const modelSelect = document.getElementById('new-model-name');

    // 创建搜索输入框和下拉列表容器
    convertToSearchableDropdown();
    
    // 设置加载中的状态
    modelSelect.innerHTML = '<option value="" disabled selected>正在加载模型列表...</option>';
    
    fetchModelList();
    
    // 内部函数，用于获取模型列表
    function fetchModelList() {
        fetch('/models-api/list')
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    // 检查返回的是对象数组还是字符串数组
                    const models = data.models;
                    let modelOptions = [];
                    let freeModels = [];
                    let giftModels = [];
                    
                    if (models && models.length > 0) {
                        if (typeof models[0] === 'object') {
                            // 对象数组情况
                            modelOptions = models;
                            freeModels = models.filter(model => model.is_free);
                            giftModels = models.filter(model => model.is_giftable);
                            
                            // 创建包含完整模型信息的对象数组
                            allModelsList = modelOptions;
                            
                            // 清除现有选项
                            modelSelect.innerHTML = '';
                            
                            // 如果没有模型，显示提示
                            if (modelOptions.length === 0) {
                                showNoModelsOptions('没有找到可用模型，请点击"同步模型"');
                                return;
                            }
                            
                            // 添加模型选项
                            modelOptions.forEach(model => {
                                const option = document.createElement('option');
                                option.value = model.id; // 使用模型的id属性作为值
                                option.textContent = model.id; // 使用模型的id属性作为显示文本
                                modelSelect.appendChild(option);
                            });
                        } else {
                            // 字符串数组情况 - 保留原有逻辑
                            modelOptions = models;
                            freeModels = models.filter(model => model.is_free);
                            giftModels = models.filter(model => model.is_giftable);
                            
                            // 更新全局模型列表数据
                            allModelsList = modelOptions.map(modelId => {
                                return {
                                    id: modelId,
                                    is_free: freeModels.includes(modelId),
                                    is_giftable: giftModels.includes(modelId) 
                                };
                            });
                            
                            // 清除现有选项
                            modelSelect.innerHTML = '';
                            
                            // 如果没有模型，显示提示
                            if (modelOptions.length === 0) {
                                showNoModelsOptions('没有找到可用模型，请点击"同步模型"');
                                return;
                            }
                            
                            // 添加模型选项
                            modelOptions.forEach(model => {
                                const option = document.createElement('option');
                                option.value = model;
                                option.textContent = model;
                                modelSelect.appendChild(option);
                            });
                        }
                    } else {
                        showNoModelsOptions('没有找到可用模型，请点击"同步模型"');
                        return;
                    }
                    
                    // 更新模型名称显示（添加免费标记）
                    updateModelNameDisplay();
                    
                    // 添加选择事件监听器
                    modelSelect.addEventListener('change', onModelSelectionChange);
                    
                    // 初始化默认策略
                    onModelSelectionChange();
                } else {
                    showNoModelsOptions(data.message || '加载模型列表失败');
                }
            })
            .catch(error => {
                console.error('获取模型列表失败:', error);
                showNoModelsOptions('获取模型列表失败，请稍后重试');
            });
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
        modelInput.placeholder = '模型名称 (例如: deepseek/deepseek-v3)';
        
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
        searchInput.placeholder = '选择或输入模型名称(红色付费,黄色赠费,绿色免费)...';
        
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
            
            // 根据输入的搜索文本筛选模型
            const filteredModels = allModelsList.filter(model => 
                String(model.id).toLowerCase().includes(searchText.toLowerCase())
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
                    item.textContent = model.id;
                    item.style.cursor = 'pointer';

                    // 根据模型属性设置不同的背景色
                    if (model.is_free) {
                        // is_free为1，设置浅绿色背景
                        item.style.backgroundColor = '#e6f7e6';
                    } else if (model.is_giftable) {
                        // is_giftable为1，设置浅黄色背景
                        item.style.backgroundColor = '#fff8e1';
                    } else if (!model.is_free && !model.is_giftable) {
                        // 两者都为0，设置浅红色背景
                        item.style.backgroundColor = '#ffebee';
                    }
                    
                    item.addEventListener('click', function() {
                        selectModel(model.id);
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
        //console.log('从API代理加载模型策略:', modelKeyStrategies);
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
    
    // 安全设置
    if (config.security) {
        setValue('password-enabled', config.security.password_enabled);
        // 不回显密码，密码字段留空
        setValue('expiration-minutes', config.security.expiration_minutes);
        // API密钥设置
        setValue('api-key-enabled', config.security.api_key_enabled);
        setValue('api-key', config.security.api_key || '');
    }
    
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
    setValue('auto-delete-zero-balance', config.app[AUTO_DELETE_ZERO_BALANCE_KEYS]);
    setValue('refresh-used-keys-interval', config.app[REFRESH_USED_KEYS_INTERVAL]);
    
    // 日志设置
    setValue('log-max-size', config.log.max_size_mb);
    setValue('log-level', config.log.level || 'warn'); // 设置日志等级，默认为warn
    
    // 请求设置
    if (config.request_settings) {
        // HTTP客户端设置
        if (config.request_settings.http_client) {
            setValue('response-header-timeout', config.request_settings.http_client.response_header_timeout);
            setValue('tls-handshake-timeout', config.request_settings.http_client.tls_handshake_timeout);
            setValue('idle-conn-timeout', config.request_settings.http_client.idle_conn_timeout);
            setValue('expect-continue-timeout', config.request_settings.http_client.expect_continue_timeout);
            setValue('max-idle-conns', config.request_settings.http_client.max_idle_conns);
            setValue('max-idle-conns-per-host', config.request_settings.http_client.max_idle_conns_per_host);
            setValue('keep-alive', config.request_settings.http_client.keep_alive);
            setValue('connect-timeout', config.request_settings.http_client.connect_timeout);
        }
        
        // 代理处理设置
        if (config.request_settings.proxy_handler) {
            setValue('inference-timeout', config.request_settings.proxy_handler.inference_timeout);
            setValue('standard-timeout', config.request_settings.proxy_handler.standard_timeout);
            setValue('stream-timeout', config.request_settings.proxy_handler.stream_timeout);
            setValue('heartbeat-interval', config.request_settings.proxy_handler.heartbeat_interval);
            setValue('progress-interval', config.request_settings.proxy_handler.progress_interval);
            setValue('buffer-threshold', config.request_settings.proxy_handler.buffer_threshold);
            setValue('max-flush-interval', config.request_settings.proxy_handler.max_flush_interval);
            setValue('max-concurrency', config.request_settings.proxy_handler.max_concurrency);
            setCheckbox('use-fake-streaming', config.request_settings.proxy_handler.use_fake_streaming);
        }
        
        // 数据库设置
        if (config.request_settings.database) {
            setValue('conn-max-lifetime', config.request_settings.database.conn_max_lifetime);
            setValue('db-max-idle-conns', config.request_settings.database.max_idle_conns);
        }
        
        // 默认值设置
        if (config.request_settings.defaults) {
            setValue('default-max-tokens', config.request_settings.defaults.max_tokens);
            setValue('default-image-size', config.request_settings.defaults.image_size);
            setValue('max-chunks-per-doc', config.request_settings.defaults.max_chunks_per_doc);
        }
    }
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
 * 获取策略文本
 * @param {number} strategy - 策略id
 * @returns {string} - 策略文本
 */
function getStrategyText(strategy) {
    switch (parseInt(strategy)) {
        case 1:
            return '策略1 - 高成功率';
        case 2:
            return '策略2 - 高分数';
        case 3:
            return '策略3 - 低RPM';
        case 4:
            return '策略4 - 低TPM';
        case 5:
            return '策略5 - 高余额';
        case 6:
            return '策略6 - 普通';
        case 7:
            return '策略7 - 低余额';
        case 8:
            return '策略8 - 免费';
        default:
            return '未知策略';
    }
}

/**
 * 编辑模型策略
 * @param {string} modelName - 模型名称
 * @param {number} strategy - 当前策略id
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
        if (allModelsList.some(model => model.id === modelName)) {
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
    // 获取模型名称和策略
    const modelNameInput = document.getElementById('new-model-name');
    const modelStrategySelect = document.getElementById('new-model-strategy');
    
    if (!modelNameInput || !modelStrategySelect) {
        showToast('找不到必要的表单元素', 'error');
        return;
    }

    const modelName = modelNameInput.value;
    if (!modelName || modelName.trim() === '') {
        showToast('请选择或输入模型名称', 'warning');
        return;
    }

    let strategyId = parseInt(modelStrategySelect.value);
    
    // 如果没有选择策略，根据模型类型设置默认策略
    if (isNaN(strategyId) || strategyId <= 0) {
        strategyId = getDefaultStrategy(modelName);
    }

    // 检查是否已有相同的模型策略配置
    const existingRow = document.querySelector(`#model-strategies-body tr[data-model="${modelName}"]`);
    if (existingRow) {
        // 如果已存在，显示编辑面板
        editModelStrategy(modelName, strategyId);
        showToast('此模型已有策略配置，已切换到编辑模式', 'info');
        return;
    }

    // 检查是否处于编辑模式
    const addButton = document.getElementById('add-model-strategy');
    const isEditing = addButton.hasAttribute('data-editing');
    
    // 如果处于编辑模式，先删除旧策略
    if (isEditing) {
        const oldModelName = addButton.getAttribute('data-editing');
        if (oldModelName !== modelName) {
            delete modelKeyStrategies[oldModelName];
        }
        // 重置按钮状态
        addButton.textContent = '添加';
        addButton.classList.remove('btn-save-modify');
        addButton.removeAttribute('data-editing');
        
        // 解除搜索框的只读状态（如果存在）
        const searchInput = document.getElementById('model-search-input');
        if (searchInput) {
            searchInput.readOnly = false;
            searchInput.style.backgroundColor = '';
            searchInput.style.cursor = '';
        }
    }

    // 更新全局变量
    modelKeyStrategies[modelName] = strategyId;
    
    // 先更新UI
    updateModelStrategiesTable();
    
    // API调用保存策略
    updateModelStrategyInDatabase(modelName, strategyId)
        .then(response => {
            if (response.success) {
                showToast(`已添加 ${modelName} 的策略配置`, 'success');
            } else {
                // 如果保存到数据库失败，回滚UI变更
                delete modelKeyStrategies[modelName];
                updateModelStrategiesTable();
                showToast(`添加失败: ${response.message}`, 'error');
            }
        })
        .catch(error => {
            console.error('添加模型策略失败:', error);
            // 如果发生错误，也回滚UI变更
            delete modelKeyStrategies[modelName];
            updateModelStrategiesTable();
            showToast(`添加失败: ${error.message}`, 'error');
        });
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
    
    // 从数据库中删除模型策略记录
    deleteModelStrategyFromDatabase(modelName)
        .then(response => {
            if (response.success) {
                showToast(`已完全移除 ${modelName} 的策略配置，并从数据库中删除`, 'success');
            } else {
                showToast(`界面上已移除策略，但从数据库删除失败: ${response.message}`, 'warning');
            }
        })
        .catch(error => {
            console.error('移除模型策略失败:', error);
            showToast(`界面上已移除策略，但从数据库删除失败`, 'warning');
        });
}

/**
 * 从数据库中删除模型策略
 * @param {string} modelId - 模型id
 * @returns {Promise} - 删除结果的Promise
 */
function deleteModelStrategyFromDatabase(modelId) {
    return fetch('/models/strategy', {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            model_id: modelId
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }
        return response.json();
    });
}

/**
 * 更新数据库中的模型策略
 * @param {string} modelId - 模型id
 * @param {number} strategyId - 策略id
 * @returns {Promise} - 更新结果的Promise
 */
function updateModelStrategyInDatabase(modelId, strategyId) {
    return fetch('/models/strategy', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            model_id: modelId,
            strategy_id: strategyId
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }
        return response.json();
    });
}

/**
 * 设置表单元素的值
 * @param {string} id - 元素id
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
 * @param {string} id - 元素id
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
        security: {
            password_enabled: getValue('password-enabled'),
            expiration_minutes: getValue('expiration-minutes'),
            api_key_enabled: getValue('api-key-enabled'),
            api_key: getValue('api-key')
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
            [RATE_REFRESH_INTERVAL]: getValue('rate-refresh'),
            [AUTO_DELETE_ZERO_BALANCE_KEYS]: getValue('auto-delete-zero-balance'),
            [REFRESH_USED_KEYS_INTERVAL]: getValue('refresh-used-keys-interval')
        },
        log: {
            max_size_mb: getValue('log-max-size'),
            level: getValue('log-level')
        },
        request_settings: {
            http_client: {
                response_header_timeout: getValue('response-header-timeout'),
                tls_handshake_timeout: getValue('tls-handshake-timeout'),
                idle_conn_timeout: getValue('idle-conn-timeout'),
                expect_continue_timeout: getValue('expect-continue-timeout'),
                max_idle_conns: getValue('max-idle-conns'),
                max_idle_conns_per_host: getValue('max-idle-conns-per-host'),
                keep_alive: getValue('keep-alive'),
                connect_timeout: getValue('connect-timeout')
            },
            proxy_handler: {
                inference_timeout: getValue('inference-timeout'),
                standard_timeout: getValue('standard-timeout'),
                stream_timeout: getValue('stream-timeout'),
                heartbeat_interval: getValue('heartbeat-interval'),
                progress_interval: getValue('progress-interval'),
                buffer_threshold: getValue('buffer-threshold'),
                max_flush_interval: getValue('max-flush-interval'),
                max_concurrency: getValue('max-concurrency')
            },
            database: {
                conn_max_lifetime: getValue('conn-max-lifetime'),
                max_idle_conns: getValue('db-max-idle-conns')
            },
            defaults: {
                max_tokens: getValue('default-max-tokens'),
                image_size: getValue('default-image-size'),
                max_chunks_per_doc: getValue('max-chunks-per-doc')
            }
        }
    };
    
    // 检查是否尝试启用API密钥验证但没有提供API密钥
    if (config.security.api_key_enabled) {
        const apiKey = getValue('api-key');
        // 检查是否有现有API密钥（通过检查api-key-enabled复选框是否已经被选中）
        const apiKeyEnabledOrig = document.getElementById('api-key-enabled').hasAttribute('data-orig-checked');
        
        // 如果没有已存在的API密钥（新启用API密钥验证）且没有提供新API密钥
        if (!apiKeyEnabledOrig && !apiKey) {
            showToast('启用API密钥验证时必须设置API密钥', 'error');
            // 聚焦API密钥输入框
            document.getElementById('api-key').focus();
            return;
        }
    }

    // 获取密码值，仅当字段不为空时才添加
    const password = getValue('password');
    if (password !== '') {
        config.security.password = password;
    }

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
            // 尝试从响应中解析错误信息
            return response.json().then(errorData => {
                throw new Error(errorData.error || `保存失败：${errorData.statusText}`);
            });
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

        // 保存初始复选框状态（用于下次验证）
        saveCheckboxOriginalState('password-enabled');
        saveCheckboxOriginalState('api-key-enabled');

        // 执行回调
        if (typeof callback === 'function') {
            callback();
        }
    })
    .catch(error => {
        console.error('保存设置失败:', error);
        // 显示优化过的错误消息，避免重复信息
        const errorMessage = error.message.replace(/保存失败：保存失败[，:]\s*/i, '');
        showToast('保存设置失败: ' + errorMessage, 'error');
    });
}

/**
 * 保存复选框的原始状态到data属性
 * @param {string} id - 复选框元素的ID
 */
function saveCheckboxOriginalState(id) {
    const checkbox = document.getElementById(id);
    if (checkbox && checkbox.checked) {
        checkbox.setAttribute('data-orig-checked', 'true');
    } else if (checkbox) {
        checkbox.removeAttribute('data-orig-checked');
    }
}

/**
 * 在DOM加载完成后保存所有复选框的初始状态
 */
function saveAllCheckboxStates() {
    saveCheckboxOriginalState('password-enabled');
    saveCheckboxOriginalState('api-key-enabled');
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

/**
 * 导出系统设置
 * 将当前系统配置导出为JSON文件下载
 */
function exportSettings() {
    try {
        // 显示正在导出的消息
        showToast('正在准备导出设置...', 'info');
        
        // 从服务器获取当前设置
        fetch('/settings/config')
            .then(response => {
                if (!response.ok) {
                    throw new Error('获取配置失败，状态码: ' + response.status);
                }
                return response.json();
            })
            .then(config => {
                // 添加版本标记，用于将来兼容性检查
                config.export_version = "v1.3.9"; // 根据当前版本号调整
                config.export_date = new Date().toISOString();
                
                // 将配置转换为JSON字符串
                const configJson = JSON.stringify(config, null, 2);
                
                // 创建Blob对象
                const blob = new Blob([configJson], { type: 'application/json' });
                
                // 创建下载链接
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.style.display = 'none';
                a.href = url;
                
                // 使用日期和时间生成文件名
                const now = new Date();
                const dateTime = now.toISOString().replace(/[:.]/g, '-').substring(0, 19);
                a.download = `flowsilicon_settings_${dateTime}.json`;
                
                // 添加到文档并触发点击事件
                document.body.appendChild(a);
                a.click();
                
                // 清理
                setTimeout(() => {
                    document.body.removeChild(a);
                    URL.revokeObjectURL(url);
                    showToast('设置已成功导出', 'success');
                }, 100);
            })
            .catch(error => {
                console.error('导出设置失败:', error);
                showToast('导出设置失败: ' + error.message, 'error');
            });
    } catch (err) {
        console.error('导出过程中发生异常:', err);
        showToast('导出过程中发生异常: ' + err.message, 'error');
    }
}

/**
 * 导入系统设置
 * @param {File} file - 要导入的设置文件
 */
function importSettings(file) {
    try {
        // 显示正在导入的消息
        showToast('正在导入设置...', 'info');
        
        const reader = new FileReader();
        
        reader.onload = function(event) {
            try {
                // 解析导入的JSON文件
                const importedConfig = JSON.parse(event.target.result);
                
                // 检查是否是有效的配置文件
                if (!importedConfig.server || !importedConfig.app) {
                    throw new Error('无效的配置文件格式');
                }
                
                // 获取当前配置作为基础，导入的设置将与当前设置合并
                fetch('/settings/config')
                    .then(response => {
                        if (!response.ok) {
                            throw new Error('获取当前配置失败，状态码: ' + response.status);
                        }
                        return response.json();
                    })
                    .then(currentConfig => {
                        // 进行配置合并，保持兼容性
                        const mergedConfig = mergeConfigs(currentConfig, importedConfig);
                        
                        // 将合并后的配置发送到服务器
                        fetch('/settings/config', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify(mergedConfig)
                        })
                        .then(response => {
                            if (!response.ok) {
                                throw new Error('导入失败，状态码: ' + response.status);
                            }
                            return response.json();
                        })
                        .then(data => {
                            showToast('设置已成功导入', 'success');
                            
                            // 重新加载设置显示
                            loadSettings();
                            
                            // 如果存在版本差异，提示用户
                            if (importedConfig.export_version && importedConfig.export_version !== "v1.3.9") {
                                showToast(`注意：导入的配置来自版本 ${importedConfig.export_version}，可能存在兼容性差异`, 'info');
                            }
                        })
                        .catch(error => {
                            console.error('导入过程中发生错误:', error);
                            showToast('导入过程中发生错误: ' + error.message, 'error');
                        });
                    })
                    .catch(error => {
                        console.error('获取当前配置失败:', error);
                        showToast('获取当前配置失败: ' + error.message, 'error');
                    });
            } catch (error) {
                console.error('解析配置文件失败:', error);
                showToast('解析配置文件失败: ' + error.message, 'error');
            }
        };
        
        reader.onerror = function() {
            console.error('读取文件失败');
            showToast('读取文件失败', 'error');
        };
        
        // 开始读取文件
        reader.readAsText(file);
    } catch (err) {
        console.error('导入过程中发生异常:', err);
        showToast('导入过程中发生异常: ' + err.message, 'error');
    }
}

/**
 * 合并配置，保持兼容性
 * @param {Object} currentConfig - 当前系统配置
 * @param {Object} importedConfig - 导入的配置
 * @returns {Object} 合并后的配置
 */
function mergeConfigs(currentConfig, importedConfig) {
    // 创建结果对象，基于当前配置
    const result = JSON.parse(JSON.stringify(currentConfig));
    
    // 递归合并配置对象
    function deepMerge(target, source) {
        // 遍历源对象的所有属性
        for (const key in source) {
            // 如果源对象的属性存在且是对象
            if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key])) {
                // 如果目标对象没有该属性或类型不同，创建新对象
                if (!target[key] || typeof target[key] !== 'object') {
                    target[key] = {};
                }
                // 递归合并
                deepMerge(target[key], source[key]);
            } else {
                // 非对象属性，直接覆盖
                target[key] = source[key];
            }
        }
    }
    
    // 合并各个配置部分
    
    // 服务器设置
    if (importedConfig.server) {
        deepMerge(result.server, importedConfig.server);
    }
    
    // API代理设置
    if (importedConfig.api_proxy) {
        if (!result.api_proxy) {
            result.api_proxy = {};
        }
        
        // 基础URL
        if (importedConfig.api_proxy.base_url) {
            result.api_proxy.base_url = importedConfig.api_proxy.base_url;
        }
        
        // 模型密钥策略
        if (importedConfig.api_proxy.model_key_strategies) {
            result.api_proxy.model_key_strategies = importedConfig.api_proxy.model_key_strategies;
            // 同时更新全局变量
            modelKeyStrategies = importedConfig.api_proxy.model_key_strategies;
        }
        
        // 重试设置
        if (importedConfig.api_proxy.retry) {
            if (!result.api_proxy.retry) {
                result.api_proxy.retry = {};
            }
            deepMerge(result.api_proxy.retry, importedConfig.api_proxy.retry);
        }
    }
    
    // 代理设置
    if (importedConfig.proxy) {
        deepMerge(result.proxy, importedConfig.proxy);
    }
    
    // 应用设置
    if (importedConfig.app) {
        if (!result.app) {
            result.app = {};
        }
        
        // 遍历导入配置中的应用设置
        for (const key in importedConfig.app) {
            // 保持兼容性：针对不同版本可能的变更，添加特殊处理
            if (key === 'model_key_strategies' && !result.api_proxy.model_key_strategies) {
                // 如果新版本将策略从app移到了api_proxy，则自动迁移
                if (!result.api_proxy) {
                    result.api_proxy = {};
                }
                result.api_proxy.model_key_strategies = importedConfig.app.model_key_strategies;
                // 同时更新全局变量
                modelKeyStrategies = importedConfig.app.model_key_strategies;
            } else {
                // 常规属性直接复制
                result.app[key] = importedConfig.app[key];
            }
        }
    }
    
    // 日志设置
    if (importedConfig.log) {
        if (!result.log) {
            result.log = {};
        }
        deepMerge(result.log, importedConfig.log);
    }
    
    // 请求设置
    if (importedConfig.request_settings) {
        if (!result.request_settings) {
            result.request_settings = {};
        }
        deepMerge(result.request_settings, importedConfig.request_settings);
    }
    
    // 跳过导出相关的元数据字段
    delete result.export_version;
    delete result.export_date;
    
    return result;
}

// 将模型名称标记为免费或付费
function updateModelNameDisplay() {
    const modelSelect = document.getElementById('new-model-name');
    const options = modelSelect.options;
    
    for (let i = 0; i < options.length; i++) {
        const option = options[i];
        const modelName = option.value;
        
        // 检查是否在allModelsList中有这个模型
        const modelInfo = allModelsList.find(model => model.id === modelName);
        if (modelInfo && modelInfo.is_free) {
            option.innerHTML = `${modelName} <span class="badge bg-success">免费</span>`;
        }
    }
}

// 根据模型是否为免费模型返回默认策略id
function getDefaultStrategy(modelName) {
    // 检查模型是否在免费模型列表中
    const modelInfo = allModelsList.find(model => model.id === modelName);
    return modelInfo && modelInfo.is_free ? 8 : 6; // 免费模型使用策略8，非免费模型使用策略6
}

// 当选择模型时自动设置默认策略
function onModelSelectionChange() {
    const modelSelect = document.getElementById('new-model-name');
    const strategySelect = document.getElementById('new-model-strategy');
    
    if (!modelSelect || !strategySelect) return;
    
    const selectedModel = modelSelect.value;
    if (selectedModel) {
        const defaultStrategy = getDefaultStrategy(selectedModel);
        strategySelect.value = defaultStrategy;
    }
}

/**
 * 生成API密钥
 * @returns {string} 生成的API密钥
 */
function generateApiKey() {
    // 创建一个随机字符串，长度为32-8=24（减去前缀"sk-"的长度）
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = 'sk-';
    
    // 生成24个随机字符
    for (let i = 0; i < 24; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    
    return result;
}

/**
 * 复制文本到剪贴板
 * @param {string} text - 要复制的文本
 * @param {boolean} isGenerated - 是否是生成的密钥
 */
function copyToClipboard(text, isGenerated) {
    // 使用现代的剪贴板API
    if (navigator.clipboard && window.isSecureContext) {
        // 如果支持Clipboard API且在安全上下文中
        navigator.clipboard.writeText(text)
            .then(() => {
                if (isGenerated) {
                    showToast('已生成新的API密钥并复制到剪贴板', 'success');
                } else {
                    showToast('已复制API密钥到剪贴板', 'success');
                }
            })
            .catch(error => {
                console.error('复制到剪贴板失败:', error);
                if (isGenerated) {
                    showToast('已生成新的API密钥，但复制到剪贴板失败', 'warning');
                } else {
                    showToast('复制到剪贴板失败', 'warning');
                }
            });
    } else {
        // 兼容性方法：创建临时文本区域
        try {
            const textArea = document.createElement('textarea');
            textArea.value = text;
            
            // 使文本区域不可见
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            textArea.style.top = '-999999px';
            document.body.appendChild(textArea);
            
            // 选择文本并复制
            textArea.focus();
            textArea.select();
            const success = document.execCommand('copy');
            
            // 清理临时元素
            document.body.removeChild(textArea);
            
            if (success) {
                if (isGenerated) {
                    showToast('已生成新的API密钥并复制到剪贴板', 'success');
                } else {
                    showToast('已复制API密钥到剪贴板', 'success');
                }
            } else {
                if (isGenerated) {
                    showToast('已生成新的API密钥，但复制到剪贴板失败', 'warning');
                } else {
                    showToast('复制到剪贴板失败', 'warning');
                }
            }
        } catch (err) {
            console.error('复制到剪贴板失败:', err);
            if (isGenerated) {
                showToast('已生成新的API密钥，但复制到剪贴板失败', 'warning');
            } else {
                showToast('复制到剪贴板失败', 'warning');
            }
        }
    }
}
