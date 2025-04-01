/**
 @author: AI
 @since: 2025/3/26 12:34:00
 @desc: 模型管理页面脚本，实现模型列表展示、筛选、状态切换和策略设置功能
 **/

// 调试模式开关
const DEBUG_MODE = true; // 设置为false可关闭调试输出

// 全局变量
const TOAST_DISPLAY_TIME = 2000; // Toast显示时间（毫秒）
let allModels = []; // 所有模型数据
let currentFilter = 'all'; // 当前筛选条件
let currentPage = 1; // 当前页码
let itemsPerPage = 20; // 每页显示数量
let isModelDisabledMap = {}; // 模型禁用状态映射

// 模型类型映射
const MODEL_TYPES = {
    1: "对话",
    2: "生图",
    3: "视频",
    4: "语音",
    5: "嵌入",
    6: "重排序",
    7: "推理"
};

// 策略映射
const STRATEGY_TYPES = {
    1: "高成功率",
    2: "高分数",
    3: "低RPM",
    4: "低TPM",
    5: "高余额",
    6: "普通",
    7: "低余额",
    8: "免费"
};

// 调试日志函数
function debug(...args) {
    if (DEBUG_MODE) {
        console.log('[DEBUG]', ...args);
    }
}

// DOM加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 加载模型数据
    loadModels();
    
    // 绑定搜索框事件
    document.getElementById('model-search').addEventListener('input', function() {
        filterModels();
    });
    
    // 绑定筛选按钮事件
    const filterButtons = document.querySelectorAll('.model-filters .btn');
    filterButtons.forEach(button => {
        button.addEventListener('click', function() {
            // 在同一按钮组内移除active类
            const parentGroup = this.closest('.btn-group');
            if (parentGroup) {
                parentGroup.querySelectorAll('.btn').forEach(btn => {
                    btn.classList.remove('active');
                });
            } else {
                // 如果不在按钮组内，移除所有按钮的active类
                filterButtons.forEach(btn => btn.classList.remove('active'));
            }
            
            this.classList.add('active');
            currentFilter = this.getAttribute('data-filter');
            currentPage = 1; // 重置到第一页
            filterModels();
        });
    });
    
    // 绑定全选/取消全选事件
    document.getElementById('select-all').addEventListener('change', function() {
        const checkboxes = document.querySelectorAll('#models-list .model-checkbox');
        checkboxes.forEach(checkbox => {
            checkbox.checked = this.checked;
        });
    });
    
    // 绑定批量禁用按钮事件
    document.getElementById('batch-disable').addEventListener('click', batchDisableModels);
    
    // 绑定同步模型按钮事件
    document.getElementById('sync-models').addEventListener('click', syncModels);
    
    // 绑定保存更改按钮事件
    document.getElementById('save-all').addEventListener('click', saveAllChanges);
    
    // 绑定返回主页按钮事件
    document.getElementById('back-to-home').addEventListener('click', function() {
        window.location.href = '/';
    });
    
    // 绑定保存编辑模型事件
    document.getElementById('save-model-edit').addEventListener('click', saveModelEdit);
    
    // 绑定自动保存切换事件
    document.getElementById('auto-save-changes').addEventListener('change', function() {
        // 保存到localStorage，使设置在会话间保持
        localStorage.setItem('autoSaveChanges', this.checked);
    });
    
    // 从localStorage加载自动保存设置
    const savedAutoSave = localStorage.getItem('autoSaveChanges') === 'true';
    document.getElementById('auto-save-changes').checked = savedAutoSave;
    
    // 添加Ctrl+S快捷键保存功能
    document.addEventListener('keydown', function(event) {
        // 检查是否为Ctrl+S组合键 (Windows) 或 Command+S (Mac)
        if ((event.ctrlKey || event.metaKey) && event.key === 's') {
            // 阻止浏览器默认的保存网页行为
            event.preventDefault();
            
            // 调用保存函数
            saveAllChanges();
            
            // 显示提示
            showToast('已通过快捷键 Ctrl+S 触发保存', 'info');
        }
    });
});

// 加载模型数据
function loadModels() {
    debug('加载模型数据');
    fetch('/models-api/list')
        .then(response => response.json())
        .then(data => {
            if (data && data.models) {
                // 统一属性名，确保所有模型对象具有相同的属性名格式
                allModels = data.models.map(model => {
                    return {
                        id: model.id,
                        type: model.type || 1,
                        is_free: model.is_free || false,
                        is_giftable: model.is_giftable || false,
                        strategy_id: model.strategy_id || 6
                    };
                });
                debug(`加载了 ${allModels.length} 个模型`);
                // 加载模型禁用状态
                loadModelStatus();
            } else {
                allModels = [];
                showToast('加载模型数据失败', 'error');
                renderModels();
            }
        })
        .catch(error => {
            console.error('加载模型数据失败:', error);
            showToast('加载模型数据失败: ' + error, 'error');
            renderModels();
        });
}

// 加载模型禁用状态
function loadModelStatus() {
    debug('加载模型禁用状态');
    fetch('/models-api/status')
        .then(response => response.json())
        .then(data => {
            if (data && data.disabled_models) {
                isModelDisabledMap = {};
                data.disabled_models.forEach(modelId => {
                    isModelDisabledMap[modelId] = true;
                });
                debug(`加载了 ${Object.keys(isModelDisabledMap).length} 个禁用模型`);
            }
            renderModels();
        })
        .catch(error => {
            console.error('加载模型状态失败:', error);
            renderModels();
        });
}

// 渲染模型列表
function renderModels() {
    const modelsList = document.getElementById('models-list');
    const searchText = document.getElementById('model-search').value.toLowerCase();
    
    // 筛选模型
    let filteredModels = allModels.filter(model => {
        // 搜索条件：检查模型ID是否包含搜索文本
        const modelName = String(model.id).toLowerCase();
        const matchesSearch = modelName.includes(searchText);
        
        // 根据当前筛选条件进行过滤
        if (currentFilter === 'all') {
            return matchesSearch;
        } else if (currentFilter === 'free') {
            return matchesSearch && model.is_free;
        } else if (currentFilter === 'giftable') {
            return matchesSearch && model.is_giftable;
        } else if (currentFilter === 'disabled') {
            return matchesSearch && isModelDisabledMap[model.id];
        } else {
            // 按类型筛选（将currentFilter转换为数字进行比较）
            return matchesSearch && model.type == parseInt(currentFilter);
        }
    });
    
    // 计算分页
    const totalModels = filteredModels.length;
    const totalPages = Math.ceil(totalModels / itemsPerPage);
    
    if (currentPage > totalPages && totalPages > 0) {
        currentPage = totalPages;
    }
    
    const start = (currentPage - 1) * itemsPerPage;
    const end = Math.min(start + itemsPerPage, totalModels);
    const paginatedModels = filteredModels.slice(start, end);
    
    // 更新显示范围和总数
    document.getElementById('current-range').textContent = totalModels > 0 ? `${start + 1}-${end}` : '0-0';
    document.getElementById('total-models').textContent = totalModels;
    
    // 清空列表
    modelsList.innerHTML = '';
    
    // 如果没有模型，显示提示
    if (paginatedModels.length === 0) {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td colspan="8" class="text-center">未找到符合条件的模型</td>`;
        modelsList.appendChild(tr);
    } else {
        // 渲染模型列表
        paginatedModels.forEach(model => {
            const isDisabled = isModelDisabledMap[model.id] || false;
            
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td><input type="checkbox" class="form-check-input model-checkbox" data-id="${model.id}"></td>
                <td class="model-id">${model.id}</td>
                <td><span class="model-type-badge type-${model.type}">${MODEL_TYPES[model.type] || '未知'}</span></td>
                <td><span class="free-tag ${model.is_free ? 'yes' : 'no'}">${model.is_free ? '是' : '否'}</span></td>
                <td><span class="giftable-tag ${model.is_giftable ? 'yes' : 'no'}">${model.is_giftable ? '是' : '否'}</span></td>
                <td><span class="strategy-tag">策略${model.strategy_id} - ${STRATEGY_TYPES[model.strategy_id] || '未知'}</span></td>
                <td><span class="status-tag ${isDisabled ? 'disabled' : 'enabled'}">${isDisabled ? '已禁用' : '已启用'}</span></td>
                <td class="action-buttons">
                    <button class="btn btn-sm btn-outline-primary edit-model" data-id="${model.id}">编辑</button>
                    <button class="btn btn-sm btn-outline-secondary copy-model khaki-btn" data-id="${model.id}">复制</button>
                    <button class="btn btn-sm ${isDisabled ? 'btn-outline-success enable-model' : 'btn-outline-danger disable-model'}" data-id="${model.id}">
                        ${isDisabled ? '启用' : '禁用'}
                    </button>
                </td>
            `;
            modelsList.appendChild(tr);
            
            // 绑定编辑按钮事件
            tr.querySelector('.edit-model').addEventListener('click', function() {
                editModel(model);
            });
            
            // 绑定复制按钮事件
            tr.querySelector('.copy-model').addEventListener('click', function() {
                copyModel(model);
            });
            
            // 绑定启用/禁用按钮事件
            const statusButton = tr.querySelector('.enable-model, .disable-model');
            statusButton.addEventListener('click', function() {
                toggleModelStatus(model.id, isDisabled);
            });
        });
    }
    
    // 渲染分页
    renderPagination(totalPages);
}

// 渲染分页控件
function renderPagination(totalPages) {
    const pagination = document.getElementById('pagination');
    pagination.innerHTML = '';
    
    if (totalPages <= 1) {
        return;
    }
    
    // 上一页
    const prevLi = document.createElement('li');
    prevLi.className = `page-item ${currentPage === 1 ? 'disabled' : ''}`;
    prevLi.innerHTML = `<a class="page-link" href="#" aria-label="上一页"><span aria-hidden="true">&laquo;</span></a>`;
    if (currentPage > 1) {
        prevLi.addEventListener('click', () => {
            currentPage--;
            renderModels();
        });
    }
    pagination.appendChild(prevLi);
    
    // 页码
    const maxVisiblePages = 5;
    let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
    let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);
    
    if (endPage - startPage + 1 < maxVisiblePages) {
        startPage = Math.max(1, endPage - maxVisiblePages + 1);
    }
    
    for (let i = startPage; i <= endPage; i++) {
        const pageLi = document.createElement('li');
        pageLi.className = `page-item ${i === currentPage ? 'active' : ''}`;
        pageLi.innerHTML = `<a class="page-link" href="#">${i}</a>`;
        pageLi.addEventListener('click', () => {
            currentPage = i;
            renderModels();
        });
        pagination.appendChild(pageLi);
    }
    
    // 下一页
    const nextLi = document.createElement('li');
    nextLi.className = `page-item ${currentPage === totalPages ? 'disabled' : ''}`;
    nextLi.innerHTML = `<a class="page-link" href="#" aria-label="下一页"><span aria-hidden="true">&raquo;</span></a>`;
    if (currentPage < totalPages) {
        nextLi.addEventListener('click', () => {
            currentPage++;
            renderModels();
        });
    }
    pagination.appendChild(nextLi);
}

// 筛选模型
function filterModels() {
    currentPage = 1; // 重置到第一页
    renderModels();
}

// 编辑模型
function editModel(model) {
    const modelEditModal = new bootstrap.Modal(document.getElementById('model-edit-modal'));
    
    // 填充表单数据
    document.getElementById('edit-model-id').value = model.id;
    document.getElementById('edit-model-type').value = model.type || 1;
    document.getElementById('edit-model-strategy').value = model.strategy_id || 6;
    document.getElementById('edit-model-free').checked = model.is_free;
    document.getElementById('edit-model-giftable').checked = model.is_giftable;
    document.getElementById('edit-model-status').checked = !isModelDisabledMap[model.id];
    
    // 更新模态框标题
    document.getElementById('model-edit-label').textContent = `编辑模型: ${model.id}`;
    
    // 显示模态框
    modelEditModal.show();
}

// 保存模型编辑
function saveModelEdit() {
    const modelId = document.getElementById('edit-model-id').value;
    const modelType = parseInt(document.getElementById('edit-model-type').value);
    const modelStrategy = parseInt(document.getElementById('edit-model-strategy').value);
    const isFree = document.getElementById('edit-model-free').checked;
    const isGiftable = document.getElementById('edit-model-giftable').checked;
    const isEnabled = document.getElementById('edit-model-status').checked;
    
    // 找到当前模型
    const modelIndex = allModels.findIndex(m => m.id === modelId);
    if (modelIndex === -1) {
        showToast('未找到模型数据', 'error');
        return;
    }
    
    // 更新模型数据
    allModels[modelIndex].type = modelType;
    allModels[modelIndex].strategy_id = modelStrategy;
    allModels[modelIndex].is_free = isFree;
    allModels[modelIndex].is_giftable = isGiftable;
    
    // 更新禁用状态
    if (isEnabled) {
        delete isModelDisabledMap[modelId];
    } else {
        isModelDisabledMap[modelId] = true;
    }
    
    // 关闭模态框
    const modelEditModal = bootstrap.Modal.getInstance(document.getElementById('model-edit-modal'));
    modelEditModal.hide();
    
    // 更新模型列表显示
    renderModels();
    
    // 立即保存更改
    saveAllChanges();
}

// 切换模型启用/禁用状态
function toggleModelStatus(modelId, currentlyDisabled) {
    if (currentlyDisabled) {
        delete isModelDisabledMap[modelId];
    } else {
        isModelDisabledMap[modelId] = true;
    }
    
    renderModels();
    
    // 立即保存更改
    saveAllChanges();
}

// 批量禁用模型
function batchDisableModels() {
    const selectedCheckboxes = document.querySelectorAll('#models-list .model-checkbox:checked');
    if (selectedCheckboxes.length === 0) {
        showToast('请选择要禁用的模型', 'info');
        return;
    }
    
    const selectedModelIds = Array.from(selectedCheckboxes).map(cb => cb.getAttribute('data-id'));
    
    // 更新禁用状态
    selectedModelIds.forEach(modelId => {
        isModelDisabledMap[modelId] = true;
    });
    
    renderModels();
    
    // 立即保存更改
    saveAllChanges();
}

// 同步模型
function syncModels() {
    showToast('正在同步模型...', 'info');
    
    fetch('/models/sync', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showToast(`模型同步成功: ${data.count} 个模型`, 'success');
                // 重新加载模型数据
                loadModels();
            } else {
                showToast(`模型同步失败: ${data.message}`, 'error');
            }
        })
        .catch(error => {
            console.error('同步模型失败:', error);
            showToast('同步模型失败: ' + error, 'error');
        });
}

// 保存所有更改
function saveAllChanges() {
    showToast('正在保存更改...', 'info');
    
    // 收集需要保存的数据
    const updates = {
        models: allModels.map(model => ({
            id: model.id,
            type: model.type,
            strategy_id: model.strategy_id,
            is_free: model.is_free,
            is_giftable: model.is_giftable
        })),
        disabled_models: Object.keys(isModelDisabledMap)
    };
    
    debug('保存更新:', updates);
    
    fetch('/models-api/update', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(updates)
    })
        .then(response => {
            if (!response.ok) {
                debug('HTTP响应错误:', response.status, response.statusText);
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            if (data.success) {
                debug('保存成功，服务器响应:', data);
                showToast('所有更改已保存', 'success');
                // 重新加载模型数据，确保显示最新状态
                loadModels();
            } else {
                debug('保存失败，服务器响应:', data);
                showToast(`保存失败: ${data.message || '未知错误'}`, 'error');
            }
        })
        .catch(error => {
            console.error('保存更改失败:', error);
            showToast('保存更改失败: ' + error, 'error');
        });
}

// 显示Toast通知
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast-notification');
    const toastTitle = document.getElementById('toast-title');
    const toastMessage = document.getElementById('toast-message');
    
    // 设置标题
    let title = '信息';
    if (type === 'success') title = '成功';
    if (type === 'error') title = '错误';
    
    toastTitle.textContent = title;
    toastMessage.textContent = message;
    
    // 设置样式
    toast.classList.remove('toast-success', 'toast-error', 'toast-info');
    toast.classList.add(`toast-${type}`);
    
    // 显示Toast
    const bsToast = new bootstrap.Toast(toast, { delay: TOAST_DISPLAY_TIME });
    bsToast.show();
}

// 复制模型
function copyModel(sourceModel) {
    // 复制模型ID到剪贴板
    navigator.clipboard.writeText(sourceModel.id)
        .then(() => {
            showToast(`已复制模型名称: ${sourceModel.id}`, 'success');
        })
        .catch(err => {
            console.error('复制到剪贴板失败:', err);
            showToast('复制模型名称失败', 'error');
        });
}
