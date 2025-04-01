/**
 @author: AI
 @since: 2025/3/29 20:00:00
 @desc: 实现登录弹窗功能
 **/

document.addEventListener('DOMContentLoaded', function() {
    // 检查是否需要显示登录弹窗
    checkLoginStatus();
});

// 检查登录状态
function checkLoginStatus() {
    // 发送请求检查登录状态
    fetch('/auth/check', {
        method: 'GET',
        credentials: 'same-origin'
    })
    .then(response => {
        if (response.status === 401) {
            // 未登录，显示登录弹窗
            showLoginModal();
            return null;
        }
        return response.json();
    })
    .then(data => {
        if (data === null) {
            return; // 已经显示了登录弹窗
        }
        // 登录成功，确保弹窗已关闭
        hideLoginModal();
    })
    .catch(error => {
        console.error('检查登录状态出错：', error);
    });
}

// 显示登录弹窗
function showLoginModal() {
    // 检查是否已存在登录弹窗
    let modal = document.getElementById('login-modal');
    if (modal) {
        // 已存在，直接显示
        const bsModal = new bootstrap.Modal(modal);
        bsModal.show();
        return;
    }

    // 创建登录弹窗
    const modalHTML = `
    <div class="modal fade" id="login-modal" tabindex="-1" data-bs-backdrop="static" data-bs-keyboard="false">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content">
                <div class="modal-header text-center">
                    <h5 class="modal-title w-100">系统登录</h5>
                </div>
                <div class="modal-body">
                    <div class="text-center mb-3">
                        <img src="/static-fs/img/logo.png" alt="Logo" style="width: 80px; margin-bottom: 15px;">
                        <p class="text-muted">请输入密码以继续访问系统</p>
                    </div>
                    <div id="login-error" class="alert alert-danger d-none">
                        <i class="bi bi-exclamation-triangle-fill me-2"></i>
                        <span id="error-message">密码错误，请重试</span>
                    </div>
                    <form id="login-modal-form">
                        <div class="form-floating mb-3">
                            <input type="password" class="form-control" id="login-password" placeholder="密码" required>
                            <label for="login-password">密码</label>
                        </div>
                        <button type="submit" class="btn btn-primary w-100 py-2">
                            <i class="bi bi-unlock me-2"></i> 登录
                        </button>
                    </form>
                </div>
            </div>
        </div>
    </div>
    `;

    // 添加到文档
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    
    // 获取新建的模态框
    modal = document.getElementById('login-modal');
    
    // 绑定提交事件
    document.getElementById('login-modal-form').addEventListener('submit', function(e) {
        e.preventDefault();
        submitLogin();
    });
    
    // 显示模态框
    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();
}

// 隐藏登录弹窗
function hideLoginModal() {
    const modal = document.getElementById('login-modal');
    if (modal) {
        const bsModal = bootstrap.Modal.getInstance(modal);
        if (bsModal) {
            bsModal.hide();
        }
    }
}

// 提交登录
function submitLogin() {
    const password = document.getElementById('login-password').value;
    
    if (!password) {
        showLoginError('请输入密码');
        return;
    }
    
    // 准备表单数据
    const formData = new FormData();
    formData.append('password', password);
    formData.append('redirect', window.location.pathname);
    
    // 发送登录请求
    fetch('/auth/login', {
        method: 'POST',
        body: formData,
        credentials: 'same-origin'
    })
    .then(response => {
        if (!response.ok) {
            if (response.status === 401) {
                showLoginError('密码错误，请重试');
            } else {
                showLoginError('登录失败，请稍后重试');
            }
            return null;
        }
        return response.json();
    })
    .then(data => {
        if (data === null) return; // 登录失败
        
        // 登录成功，隐藏弹窗并刷新页面
        hideLoginModal();
        window.location.reload();
    })
    .catch(error => {
        console.error('登录请求出错：', error);
        showLoginError('网络错误，请稍后重试');
    });
}

// 显示登录错误信息
function showLoginError(message) {
    const errorDiv = document.getElementById('login-error');
    const errorMsg = document.getElementById('error-message');
    
    errorMsg.textContent = message;
    errorDiv.classList.remove('d-none');
    
    // 3秒后自动隐藏错误信息
    setTimeout(() => {
        errorDiv.classList.add('d-none');
    }, 3000);
} 