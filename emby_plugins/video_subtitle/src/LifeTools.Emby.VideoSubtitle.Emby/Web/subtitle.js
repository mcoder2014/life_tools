define([], function () {
    'use strict';

    function View(view, params) {

        var pluginId = 'C62D8714-7F3C-49F0-B4BB-A1B2D9C77A55';
        var page = view;
        var selected = {};
        var initialized = false;

        function byId(id) { return page.querySelector('#' + id); }
        function closestById(el, id) { return el && el.closest ? el.closest('#' + id) : null; }
        function configMessage(text, cls) { setMessage(page.querySelector('[data-ltvs-message="config"]'), 'ltvs-config-message', text, cls); }
        function submitMessage(text, cls) { setMessage(page.querySelector('[data-ltvs-message="submit"]'), 'ltvs-submit-message', text, cls); }
        function jobsMessage(text, cls) { setMessage(page.querySelector('[data-ltvs-message="jobs"]'), 'ltvs-jobs-message', text, cls); }
        function setMessage(el, baseClass, text, cls) {
            if (!el) return;
            el.className = baseClass + ' ltvs-message ' + (cls || '');
            el.textContent = text || '';
        }
        function html(value) {
            return String(value == null ? '' : value).replace(/[&<>"]/g, function (c) {
                return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];
            });
        }
        function apiJson(path, method, data, params) {
            method = method || 'GET';
            var options = { url: ApiClient.getUrl(path, params || null), type: method, dataType: 'json' };
            options.timeout = 30000;
            var hasBody = data !== undefined && method !== 'GET' && method !== 'HEAD';
            if (hasBody) {
                options.data = JSON.stringify(data);
                options.contentType = 'application/json';
            }
            return ApiClient.ajax(options).then(null, function (err) {
                if (err && typeof err.text === 'function') {
                    return err.text().then(function (body) {
                        throw new Error(body || err.statusText || ('HTTP ' + err.status));
                    });
                }
                throw err;
            });
        }
        function loadConfig() {
            return ApiClient.getPluginConfiguration(pluginId).then(function (config) {
                byId('ltvsExecutablePath').value = config.ExecutablePath || '/usr/local/bin/video_subtitle';
                byId('ltvsConfigPath').value = config.ConfigPath || '/etc/life_tools/video_subtitle.json';
                byId('ltvsDefaultSourceLanguage').value = config.DefaultSourceLanguage || 'ja-JP';
                byId('ltvsSourceLanguage').value = config.DefaultSourceLanguage || 'ja-JP';
                byId('ltvsExtraArgs').value = config.ExtraArgs || '';
                byId('ltvsMaxLogTailBytes').value = config.MaxLogTailBytes || 8192;
            }, function (err) {
                configMessage('读取配置失败：' + errorText(err), 'ltvs-error');
            });
        }
        function saveConfig(e) {
            e.preventDefault();
            configMessage('保存中...');
            var config = {
                ExecutablePath: byId('ltvsExecutablePath').value.trim(),
                ConfigPath: byId('ltvsConfigPath').value.trim(),
                DefaultSourceLanguage: byId('ltvsDefaultSourceLanguage').value.trim(),
                ExtraArgs: byId('ltvsExtraArgs').value,
                MaxLogTailBytes: parseInt(byId('ltvsMaxLogTailBytes').value || '8192', 10)
            };
            ApiClient.updatePluginConfiguration(pluginId, config).then(function () {
                byId('ltvsSourceLanguage').value = config.DefaultSourceLanguage || 'ja-JP';
                configMessage('已保存', 'ltvs-success');
            }, function (err) {
                configMessage('保存失败：' + errorText(err), 'ltvs-error');
            });
        }
        function loadRootTree() {
            var tree = byId('ltvsTree');
            tree.innerHTML = '<div class="ltvs-tree-row"><span></span><span>加载媒体库...</span><span></span></div>';
            return ApiClient.getUserViews({}, ApiClient.getCurrentUserId()).then(function (result) {
                tree.innerHTML = '';
                (result.Items || []).forEach(function (item) { renderItem(tree, item, 0); });
                if (!tree.children.length) tree.innerHTML = '<div class="ltvs-tree-row"><span></span><span>没有可显示的媒体库</span><span></span></div>';
            }, function (err) {
                tree.innerHTML = '<div class="ltvs-tree-row"><span></span><span>加载失败：' + html(errorText(err)) + '</span><span></span></div>';
            });
        }
        function renderItem(container, item, depth) {
            var row = document.createElement('div');
            row.className = 'ltvs-tree-row';
            row.style.paddingLeft = Math.min(depth * 18, 72) + 'px';
            row.dataset.itemId = item.Id || '';
            var isVideo = item.MediaType === 'Video';
            var canExpand = item.IsFolder || item.Type === 'UserView' || item.Type === 'CollectionFolder' || item.Type === 'Folder' || item.CollectionType;
            row.innerHTML = '<button type="button" class="ltvs-expand">' + (canExpand ? '+' : '') + '</button>' +
                '<div class="ltvs-tree-name" title="' + html(item.Name) + '">' + html(item.Name || item.Id) + '<span class="ltvs-tree-meta">' + html(item.Type || item.MediaType || '') + '</span></div>' +
                '<label>' + (isVideo ? '<input type="checkbox" class="ltvs-select">' : '') + '</label>';
            container.appendChild(row);
            if (canExpand) {
                row.querySelector('.ltvs-expand').addEventListener('click', function () { toggleChildren(row, item, depth); });
            }
            if (isVideo) {
                row.querySelector('.ltvs-select').addEventListener('change', function (e) {
                    if (e.target.checked) selected[item.Id] = item;
                    else delete selected[item.Id];
                    updateSelectedCount();
                });
            }
        }
        function toggleChildren(row, item, depth) {
            if (row.dataset.loaded === '1') {
                row.dataset.open = row.dataset.open === '1' ? '0' : '1';
                setChildrenVisible(row, row.dataset.open === '1');
                row.querySelector('.ltvs-expand').textContent = row.dataset.open === '1' ? '-' : '+';
                return;
            }
            row.dataset.loaded = '1';
            row.dataset.open = '1';
            row.querySelector('.ltvs-expand').textContent = '-';
            ApiClient.getItems(ApiClient.getCurrentUserId(), {
                ParentId: item.Id,
                SortBy: 'SortName',
                SortOrder: 'Ascending',
                EnableTotalRecordCount: false,
                EnableImages: false,
                EnableUserData: false,
                Fields: 'BasicSyncInfo,Path'
            }).then(function (result) {
                var after = row;
                (result.Items || []).forEach(function (child) {
                    after = renderItemAfter(after, child, depth + 1, row.dataset.itemId);
                });
            }, function (err) {
                submitMessage('加载子项失败：' + errorText(err), 'ltvs-error');
            });
        }
        function renderItemAfter(after, item, depth, parentId) {
            var container = after.parentNode;
            var marker = document.createElement('div');
            container.insertBefore(marker, after.nextSibling);
            renderItem(container, item, depth);
            var rendered = container.lastElementChild;
            rendered.dataset.parentId = parentId;
            container.insertBefore(rendered, marker);
            container.removeChild(marker);
            return rendered;
        }
        function setChildrenVisible(row, visible) {
            var id = row.dataset.itemId;
            Array.prototype.forEach.call(byId('ltvsTree').querySelectorAll('[data-parent-id="' + id + '"]'), function (child) {
                child.style.display = visible ? '' : 'none';
                if (!visible || child.dataset.open === '1') {
                    setChildrenVisible(child, visible && child.dataset.open === '1');
                }
            });
        }
        function updateSelectedCount() {
            byId('ltvsSelectedCount').textContent = '已选择 ' + Object.keys(selected).length + ' 个视频';
        }
        function submitBatch() {
            var sourceLanguage = byId('ltvsSourceLanguage').value.trim();
            var common = {
                SourceLanguage: sourceLanguage,
                ForceAsr: byId('ltvsForceAsr').checked,
                ForceSplit: byId('ltvsForceSplit').checked,
                ForceTranslate: byId('ltvsForceTranslate').checked,
                ForceRequeue: byId('ltvsForceRequeue').checked
            };
            var missingPathNames = [];
            var items = Object.keys(selected).map(function (id) {
                var item = selected[id];
                var path = item.Path || '';
                if (!path) missingPathNames.push(item.Name || id);
                return Object.assign({ ItemId: id, VideoPath: path }, common);
            }).filter(function (item) { return item.VideoPath; });
            if (missingPathNames.length) {
                submitMessage('选中的视频缺少本地路径，无法提交：' + missingPathNames.slice(0, 3).join('、'), 'ltvs-error');
                return;
            }
            byId('ltvsManualPaths').value.split(/\r?\n/).map(function (line) { return line.trim(); }).filter(Boolean).forEach(function (path) {
                items.push(Object.assign({ VideoPath: path }, common));
            });
            if (!items.length) {
                submitMessage('请先选择视频或填写视频路径', 'ltvs-error');
                return;
            }
            submitMessage('提交中...');
            apiJson('LifeTools/VideoSubtitle/Batches', 'POST', { ForceRequeue: common.ForceRequeue, Items: items }).then(function (result) {
                var jobs = result && (result.Jobs || result.jobs) || [];
                submitMessage('已提交 ' + jobs.length + ' 个任务', 'ltvs-success');
                renderJobs(jobs, '已提交任务，等待刷新历史');
                loadJobs();
                setTimeout(loadJobs, 1200);
            }, function (err) {
                submitMessage('提交失败：' + errorText(err), 'ltvs-error');
            });
        }
        function renderJobs(jobs, emptyText) {
            var body = byId('ltvsJobsBody');
            body.innerHTML = '';
            if (!jobs || !jobs.length) {
                body.innerHTML = '<tr><td colspan="5">' + html(emptyText || '暂无数据') + '</td></tr>';
                return;
            }
            jobs.forEach(function (job) {
                var tr = document.createElement('tr');
                var canCancel = job.Status === 'Queued' || job.Status === 'Running' || job.Status === 'CancelRequested';
                tr.innerHTML = '<td>' + html(job.Status) + (job.ErrorMessage ? '<div class="ltvs-error">' + html(job.ErrorMessage) + '</div>' : '') + '</td>' +
                    '<td class="ltvs-path" title="' + html(job.VideoPath) + '">' + html(job.VideoPath) + '</td>' +
                    '<td class="ltvs-path" title="' + html(job.OutputPath || '') + '">' + html(job.OutputPath || '') + '</td>' +
                    '<td>' + html(formatDate(job.CreatedAt)) + '</td>' +
                    '<td>' + (canCancel ? '<button is="emby-button" type="button" data-job-id="' + html(job.JobId) + '"><span>取消</span></button>' : '') + '</td>';
                body.appendChild(tr);
            });
            Array.prototype.forEach.call(body.querySelectorAll('button[data-job-id]'), function (button) {
                button.addEventListener('click', function () { cancelJob(button.getAttribute('data-job-id')); });
            });
        }
        function loadJobs() {
            jobsMessage('加载中...');
            renderJobs([], '加载中...');
            return apiJson('LifeTools/VideoSubtitle/Jobs', 'GET', undefined, { Limit: 50 }).then(function (jobs) {
                jobsMessage('');
                renderJobs(jobs, '暂无数据');
            }, function (err) {
                jobsMessage('加载历史失败：' + errorText(err), 'ltvs-error');
                byId('ltvsJobsBody').innerHTML = '<tr><td colspan="5">加载历史失败</td></tr>';
            });
        }
        function cancelJob(id) {
            if (!id) return;
            jobsMessage('取消中...');
            apiJson('LifeTools/VideoSubtitle/Jobs/' + encodeURIComponent(id) + '/Cancel', 'POST').then(loadJobs, function (err) {
                jobsMessage('取消失败：' + errorText(err), 'ltvs-error');
            });
        }
        function formatDate(value) {
            if (!value) return '';
            var date = new Date(value);
            return isNaN(date.getTime()) ? value : date.toLocaleString();
        }
        function errorText(err) {
            return (err && (err.responseText || err.statusText || err.message)) || String(err || '未知错误');
        }
        function init() {
            if (initialized) return;
            initialized = true;
            page.querySelector('.ltvs-config-form').onsubmit = saveConfig;
            byId('ltvsSubmit').onclick = function (e) {
                e.preventDefault();
                submitBatch();
            };
            byId('ltvsRefreshJobs').onclick = function (e) {
                e.preventDefault();
                loadJobs();
            };
            loadConfig();
            loadRootTree();
            loadJobs();
        }

        setTimeout(init, 0);

        this.onResume = function (options) {
            if (initialized) {
                loadJobs();
                return;
            }
            init();
        };
    }

    View.prototype.onPause = function () {};
    return View;
});
