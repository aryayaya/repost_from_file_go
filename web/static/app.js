document.addEventListener('DOMContentLoaded', () => {
    // DOM 元素缓存
    const tasksContainer = document.getElementById('tasksContainer');
    const taskCountBadge = document.getElementById('taskCountBadge');
    const logSizeDisplay = document.getElementById('logSizeDisplay');
    const clearLogBtn = document.getElementById('clearLogBtn');
    
    const fileUploadInput = document.getElementById('fileUploadInput');
    const uploadLabelBtn = document.getElementById('uploadLabelBtn');
    const uploadFeedback = document.getElementById('uploadFeedback');
    const uploadProgressText = document.getElementById('uploadProgressText');
    
    const replayAllBtn = document.getElementById('replayAllBtn');
    const removeAllBtn = document.getElementById('removeAllBtn');
    
    const batchReplayFeedback = document.getElementById('replayAllFeedback');
    const batchReplayText = document.getElementById('batchReplayText');
    const batchReplaySpinner = document.getElementById('batchReplaySpinner');
    const batchReplayDoneIcon = document.getElementById('batchReplayDoneIcon');

    // 本地状态：记录在内存中的卡片 DOM
    let taskCards = []; 

    // 初始化加载
    loadTasks();
    loadLogSize();

    // =============== 事件监听 ===============

    // 1. 获取日志大小
    async function loadLogSize() {
        try {
            const res = await fetch('/api/history?_t=' + Date.now());
            // 如果接口返回 404 或内容为空，大小为 0
            if (!res.ok) {
                logSizeDisplay.textContent = '0 Bytes';
                return;
            }
            
            // 后端 /api/history 目前返回 JSON 数组，我们要的是日志大小
            // 这里为了最简单的实现，我们可以读取 JSON 字符串的长度，粗略等同于字节大小
            const text = await res.text();
            if (text.length <= 2 || text === '[]') { // '[]'
                logSizeDisplay.textContent = '0 Bytes';
            } else {
                const kb = (text.length / 1024).toFixed(2);
                logSizeDisplay.textContent = `${kb} KB`;
            }
        } catch (err) {
            logSizeDisplay.textContent = 'Error';
        }
    }

    // 2. 清空历史
    clearLogBtn.addEventListener('click', async () => {
        if (!confirm('Are you sure you want to clear the system log (history.json)?')) return;
        
        try {
            // 需要后端提供一个 clear 的接口。如果还没有实现，可以新增 POST /api/clear_history
            const res = await fetch('/api/clear_history', { method: 'POST' });
            if (res.ok) {
                loadLogSize();
            }
        } catch (err) {
            console.error(err);
        }
    });

    // 3. 多选上传文件
    fileUploadInput.addEventListener('change', async (e) => {
        const files = e.target.files;
        if (!files || files.length === 0) return;

        // UI 切换到上传中模式
        uploadLabelBtn.style.display = 'none';
        uploadFeedback.style.display = 'inline-flex';
        
        let successCount = 0;
        const total = files.length;

        for (let i = 0; i < total; i++) {
            const file = files[i];
            uploadProgressText.textContent = `Uploading ${i+1}/${total}`;
            
            const formData = new FormData();
            formData.append('file', file);

            try {
                const res = await fetch('/api/upload', {
                    method: 'POST',
                    body: formData
                });
                if (res.ok) successCount++;
            } catch (err) {
                console.error('Upload error:', err);
            }
        }

        // 上传完成 UI
        uploadFeedback.innerHTML = `<i data-feather="check-circle" class="text-success"></i><span>${successCount}/${total} Completed</span>`;
        feather.replace();
        
        // 3秒后恢复原始按钮
        setTimeout(() => {
            uploadFeedback.style.display = 'none';
            uploadFeedback.innerHTML = `<i data-feather="loader" class="spin"></i><span id="uploadProgressText">0/0</span>`;
            uploadLabelBtn.style.display = 'inline-flex';
            fileUploadInput.value = ''; // reset
            loadTasks(); 
        }, 3000);
    });

    // 4. 重放全部
    replayAllBtn.addEventListener('click', async () => {
        const visibleTasks = Array.from(tasksContainer.children).filter(c => !c.classList.contains('empty-state') && !c.classList.contains('hidden'));
        if (visibleTasks.length === 0) return;

        replayAllBtn.disabled = true;
        
        // 展示 Banner
        batchReplayFeedback.style.display = 'block';
        batchReplayFeedback.className = 'batch-feedback-banner';
        batchReplaySpinner.style.display = 'inline-block';
        batchReplayDoneIcon.style.display = 'none';
        batchReplayText.textContent = `Batch Replaying ${visibleTasks.length} tasks... Please wait.`;

        // 并发触发（直接调用后端现成的 /api/replay_all）
        // 注意：后端的 replay_all 是读取所有存在的文件。
        // （纯前端移除的这里还是会被重放。这是业务取舍，我们暂时遵循后端逻辑全量重放）
        try {
            const res = await fetch('/api/replay_all', { method: 'POST' });
            const data = await res.json();
            
            if (res.ok) {
                batchReplaySpinner.style.display = 'none';
                batchReplayDoneIcon.style.display = 'inline-block';
                
                const total = data.success + data.failed;
                batchReplayText.textContent = `Complete: ${data.success}/${total} success, ${data.failed} failed.`;
                
                // 遍历后端返回的每一个文件的详细结果并更新 DOM 卡片
                if (data.results && Array.isArray(data.results)) {
                    data.results.forEach(result => {
                        // 通过 data-filename 寻找对应的展示卡片
                        const card = tasksContainer.querySelector(`.task-card[data-filename="${CSS.escape(result.Filename)}"]`);
                        if (card) {
                            const statusDiv = card.querySelector('.status-indicator');
                            const snippetDiv = card.querySelector('.error-snippet-container');
                            
                            if (statusDiv) {
                                statusDiv.innerHTML = `<span>${result.StatusCode}</span>`;
                                if (result.Success) {
                                    statusDiv.className = 'status-indicator success';
                                    if (snippetDiv) {
                                        snippetDiv.style.display = 'none';
                                        snippetDiv.innerHTML = '';
                                    }
                                } else {
                                    statusDiv.className = 'status-indicator error';
                                    if (snippetDiv) {
                                        snippetDiv.style.display = 'block';
                                        snippetDiv.className = 'error-snippet';
                                        const errText = result.ResponseText || result.ErrMsg || 'Unknown Error';
                                        snippetDiv.textContent = errText;
                                        snippetDiv.title = errText;
                                    }
                                }
                            }
                        }
                    });
                }
                
                if (data.failed > 0) {
                    batchReplayFeedback.classList.add('warning');
                    loadLogSize();
                } else {
                    batchReplayFeedback.classList.add('success');
                }
            }
        } catch (err) {
            batchReplaySpinner.style.display = 'none';
            batchReplayText.textContent = `Network Error: ${err.message}`;
            batchReplayFeedback.classList.add('warning');
        } finally {
            replayAllBtn.disabled = false;
            feather.replace();
            
            // 全局重放后刷新卡片列表状态（或者直接粗暴 loadTasks() 重新拉取但会丢失本会话的状态码，所以这里不重刷列标，维持现状）
            // 用户可以单点调试
        }
    });

    // 5. 前端清空全部卡片视图
    removeAllBtn.addEventListener('click', () => {
        if (!confirm('Remove all cards from current view? (Source files will NOT be deleted)')) return;
        tasksContainer.innerHTML = createEmptyState();
        taskCountBadge.textContent = '0';
        feather.replace();
    });

    // =============== 核心列表与卡片逻辑 ===============

    async function loadTasks() {
        try {
            const res = await fetch('/api/tasks');
            const tasks = await res.json();

            if (!tasks || tasks.length === 0) {
                tasksContainer.innerHTML = createEmptyState();
                taskCountBadge.textContent = '0';
                feather.replace();
                return;
            }

            taskCountBadge.textContent = tasks.length;
            tasksContainer.innerHTML = '';
            
            tasks.forEach(task => {
                const card = createTaskCard(task);
                tasksContainer.appendChild(card);
            });
            feather.replace();
        } catch (err) {
            console.error('Failed to load tasks', err);
        }
    }

    function createEmptyState() {
        return `
            <div class="empty-state" style="grid-column: 1 / -1; padding: 4rem;">
                <i data-feather="inbox" style="width: 48px; height: 48px; color: #ccc;"></i>
                <h3 style="margin-top: 1rem; color: #888;">No requests in view</h3>
                <p style="color: #bbb; margin-top: 8px;">Upload .txt files to get started.</p>
            </div>`;
    }

    // 构建单张任务卡片 DOM
    function createTaskCard(task) {
        const div = document.createElement('div');
        div.className = 'task-card';
        div.dataset.filename = task.filename;
        
        const methodClass = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].includes(task.method) 
            ? `method-${task.method}` 
            : 'method-DEFAULT';

        div.innerHTML = `
            <div class="card-header">
                <span class="method-badge ${methodClass}">${task.method}</span>
                <span class="filename" title="${task.filename}">${task.filename}</span>
            </div>
            
            <div class="url-box" title="${task.url}">${task.url}</div>
            
            <div class="error-snippet-container" style="display: none;"></div>

            <div class="card-footer" style="margin-top: 16px;">
                <!-- 状态码指示器 (首次呈现双横杠) -->
                <div class="status-indicator">
                    <span>--</span>
                </div>
                
                <div class="card-actions">
                    <!-- 纯视觉移除按钮 -->
                    <button class="card-icon-btn delete-btn" title="Remove from View">
                        <i data-feather="trash-2" style="width:16px; height:16px;"></i>
                    </button>
                    <!-- 单次播放按钮 -->
                    <button class="card-icon-btn play-btn" title="Replay Request">
                        <i data-feather="play"></i>
                    </button>
                </div>
            </div>
        `;

        // 绑定单次播放
        const playBtn = div.querySelector('.play-btn');
        const statusDiv = div.querySelector('.status-indicator');
        const snippetDiv = div.querySelector('.error-snippet-container');
        
        playBtn.addEventListener('click', async () => {
            playBtn.classList.add('loading');
            statusDiv.innerHTML = `<i data-feather="loader" class="spin" style="width:16px;height:16px;"></i>`;
            feather.replace();
            
            try {
                const res = await fetch(`/api/replay/${encodeURIComponent(task.filename)}`, { method: 'POST' });
                const data = await res.json();
                
                statusDiv.innerHTML = `<span>${data.StatusCode}</span>`;
                if (data.Success) {
                    statusDiv.className = 'status-indicator success';
                    snippetDiv.style.display = 'none';
                    snippetDiv.innerHTML = '';
                } else {
                    statusDiv.className = 'status-indicator error';
                    loadLogSize(); // 失败后更新左侧挂件大小
                    
                    // 展示错误摘要
                    snippetDiv.style.display = 'block';
                    snippetDiv.className = 'error-snippet';
                    // 优先取 ResponseContext(如果是 HTTP 请求真的发出去报错了)，其次是 ErrMsg(网络错误等导致没发出去)
                    const errText = data.ResponseText || data.ErrMsg || 'Unknown Error';
                    snippetDiv.textContent = errText;
                    snippetDiv.title = errText; // 悬浮显示完整内容
                }
            } catch (err) {
                statusDiv.innerHTML = `<span>ERR</span>`;
                statusDiv.className = 'status-indicator error';
            } finally {
                playBtn.classList.remove('loading');
            }
        });

        // 绑定单张卡片隐藏
        const deleteBtn = div.querySelector('.delete-btn');
        deleteBtn.addEventListener('click', () => {
            div.remove();
            // 不调用后端真实删除
            const currentCount = parseInt(taskCountBadge.textContent, 10);
            taskCountBadge.textContent = Math.max(0, currentCount - 1);
            
            if (tasksContainer.children.length === 0) {
                tasksContainer.innerHTML = createEmptyState();
                feather.replace();
            }
        });

        return div;
    }
});
