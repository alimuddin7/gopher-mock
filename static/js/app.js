// Global helper: opens a templui dialog by its container ID
function openDialog(containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.setAttribute('data-tui-dialog-open', 'true');
    const dlg = container.querySelector('dialog[data-tui-dialog-content]');
    if (dlg) {
        dlg.setAttribute('data-tui-dialog-open', 'true');
        if (!dlg.open) dlg.showModal();
    }
}

// Global helper: closes a templui dialog by its container ID
function closeDialog(containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.setAttribute('data-tui-dialog-open', 'false');
    const dlg = container.querySelector('dialog[data-tui-dialog-content]');
    if (dlg) {
        dlg.setAttribute('data-tui-dialog-open', 'false');
        dlg.close();
    }
}

function configManager() {
    return {
        configs: [],
        selectedIndex: null,
        searchQuery: '',
        activeTab: 'response',
        showSidebar: false,
        modalMessage: '',
        errorTitle: '',
        errorMessage: '',
        importFile: null,
        importFileName: '',
        importMerge: true,
        importLoading: false,
        selectedIndices: [],
        isBulkDelete: false,
        isDark: document.documentElement.classList.contains('dark'),

        toggleTheme() {
            this.isDark = !this.isDark;
            if (this.isDark) {
                document.documentElement.classList.add('dark');
            } else {
                document.documentElement.classList.remove('dark');
            }
        },

        showError(title, msg) {
            this.errorTitle = title;
            this.errorMessage = msg;
            openDialog('error_modal');
        },

        get selectedConfig() {
            return this.selectedIndex !== null ? this.configs[this.selectedIndex] : null;
        },

        getMethodBadgeClass(method) {
            return {
                'text-blue-500 bg-blue-500/10 border-blue-500/20': method === 'GET',
                'text-emerald-500 bg-emerald-500/10 border-emerald-500/20': method === 'POST',
                'text-amber-500 bg-amber-500/10 border-amber-500/20': method === 'PUT',
                'text-red-500 bg-red-500/10 border-red-500/20': method === 'DELETE',
                'text-muted-foreground bg-muted border-border': !['GET', 'POST', 'PUT', 'DELETE'].includes(method)
            };
        },

        get filteredConfigs() {
            const query = this.searchQuery.toLowerCase();
            return this.configs
                .map((cfg, index) => ({ ...cfg, originalIndex: index }))
                .filter(cfg => {
                    if (!query) return true;
                    return cfg.name.toLowerCase().includes(query) ||
                        cfg.path.toLowerCase().includes(query) ||
                        cfg.method.toLowerCase().includes(query);
                });
        },

        get groupedConfigs() {
            const filtered = this.filteredConfigs;
            const groups = {}; // Key: directory path, Value: array of configs
            const rootConfigs = [];

            filtered.forEach(cfg => {
                const parts = cfg.path.split('/').filter(p => p.length > 0);
                if (parts.length <= 1) {
                    rootConfigs.push(cfg);
                } else {
                    const groupPath = '/' + parts.slice(0, -1).join('/');
                    if (!groups[groupPath]) {
                        groups[groupPath] = [];
                    }
                    groups[groupPath].push(cfg);
                }
            });

            const tree = Object.keys(groups).sort().map(path => ({
                name: path,
                fullPath: path,
                configs: groups[path].sort((a, b) => a.name.localeCompare(b.name))
            }));

            return {
                rootConfigs: rootConfigs.sort((a, b) => a.name.localeCompare(b.name)),
                tree: tree
            };
        },

        selectConfig(index) {
            this.selectedIndex = index;
            this.showSidebar = false;
        },

        toggleSelection(index) {
            const idx = this.selectedIndices.indexOf(index);
            if (idx > -1) {
                this.selectedIndices.splice(idx, 1);
            } else {
                this.selectedIndices.push(index);
            }
        },

        get isAllSelected() {
            const filtered = this.filteredConfigs;
            return filtered.length > 0 && filtered.every(cfg => this.selectedIndices.includes(cfg.originalIndex));
        },

        toggleSelectAll() {
            const filteredIndices = this.filteredConfigs.map(cfg => cfg.originalIndex);
            if (this.isAllSelected) {
                this.selectedIndices = this.selectedIndices.filter(id => !filteredIndices.includes(id));
            } else {
                this.selectedIndices = [...new Set([...this.selectedIndices, ...filteredIndices])];
            }
        },

        addNewConfig() {
            const newConfig = {
                name: 'New Configuration',
                method: 'GET',
                path: '/api/new',
                statusCode: 200,
                timeout: 0,
                requestHeaders: '{}',
                requestBody: '{}',
                responseHeaders: '{}',
                responseBody: '{}',
                responses: [],
                defaultResponse: {
                    statusCode: 200,
                    timeout: 0,
                    headers: '{}',
                    body: '{}'
                }
            };
            this.configs.push(newConfig);
            this.selectedIndex = this.configs.length - 1;
        },

        addConditionalResponse() {
            if (!this.selectedConfig.responses) {
                this.selectedConfig.responses = [];
            }
            this.selectedConfig.responses.push({
                name: 'New Response',
                ruleOperator: 'AND',
                rules: [],
                response: {
                    statusCode: 200,
                    timeout: 0,
                    headers: '{}',
                    body: '{}'
                }
            });
        },

        deleteConditionalResponse(index) {
            this.selectedConfig.responses.splice(index, 1);
        },

        addRule(responseIndex) {
            if (!this.selectedConfig.responses[responseIndex].rules) {
                this.selectedConfig.responses[responseIndex].rules = [];
            }
            this.selectedConfig.responses[responseIndex].rules.push({
                target: 'body',
                field: '',
                operator: 'equals',
                value: ''
            });
        },

        deleteRule(responseIndex, ruleIndex) {
            this.selectedConfig.responses[responseIndex].rules.splice(ruleIndex, 1);
        },

        beautifyConditionalJSON(responseIndex, field) {
            try {
                const resp = this.selectedConfig.responses[responseIndex].response;
                const parsed = JSON.parse(resp[field]);
                resp[field] = JSON.stringify(parsed, null, 2);
            } catch (e) {
                this.showError('Invalid JSON', "Could not beautify " + field + " JSON.");
            }
        },

        duplicateConfig(index) {
            const original = this.configs[index];
            const duplicate = JSON.parse(JSON.stringify(original));
            duplicate.name = duplicate.name + ' (Copy)';
            duplicate.path = duplicate.path + '-copy';
            this.configs.push(duplicate);
            this.selectedIndex = this.configs.length - 1;
        },

        deleteCurrentConfig() {
            if (this.selectedIndex === null) return;
            this.isBulkDelete = false;
            openDialog('delete_modal');
        },

        deleteSelectedConfigs() {
            if (this.selectedIndices.length === 0) return;
            this.isBulkDelete = true;
            openDialog('delete_modal');
        },

        confirmDelete() {
            if (this.isBulkDelete) {
                fetch('/delete-configs', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ indices: this.selectedIndices })
                }).then(res => res.ok && location.reload());
                return;
            }

            if (this.selectedIndex === null) return;
            fetch('/delete-config/' + this.selectedIndex, { method: 'POST' })
                .then(res => res.ok && location.reload());
        },

        beautifyJSON(field) {
            try {
                const parsed = JSON.parse(this.selectedConfig[field]);
                this.selectedConfig[field] = JSON.stringify(parsed, null, 2);
            } catch (e) {
                this.showError('Invalid JSON', "Could not beautify " + field + " JSON.");
            }
        },

        saveAll() {
            try {
                const configsToSave = this.configs.map(cfg => {
                    const result = {
                        name: cfg.name,
                        method: cfg.method,
                        path: cfg.path,
                        statusCode: parseInt(cfg.statusCode) || 200,
                        timeout: parseInt(cfg.timeout) || 0,
                        requestHeaders: JSON.parse(cfg.requestHeaders || '{}'),
                        requestBody: JSON.parse(cfg.requestBody || '{}'),
                        responseHeaders: JSON.parse(cfg.responseHeaders || '{}'),
                        responseBody: JSON.parse(cfg.responseBody || '{}')
                    };

                    if (cfg.responses && cfg.responses.length > 0) {
                        result.responses = cfg.responses.map(resp => ({
                            name: resp.name,
                            ruleOperator: resp.ruleOperator,
                            rules: resp.rules,
                            response: {
                                statusCode: parseInt(resp.response.statusCode) || 200,
                                timeout: parseInt(resp.response.timeout) || 0,
                                headers: JSON.parse(resp.response.headers || '{}'),
                                body: JSON.parse(resp.response.body || '{}')
                            }
                        }));
                    }

                    if (cfg.defaultResponse) {
                        result.defaultResponse = {
                            statusCode: parseInt(cfg.defaultResponse.statusCode) || 200,
                            timeout: parseInt(cfg.defaultResponse.timeout) || 0,
                            headers: JSON.parse(cfg.defaultResponse.headers || '{}'),
                            body: JSON.parse(cfg.defaultResponse.body || '{}')
                        };
                    }

                    return result;
                });

                fetch('/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(configsToSave)
                })
                .then(res => {
                    if (!res.ok) return res.text().then(t => { throw new Error(t) });
                    return res.text();
                })
                .then(msg => {
                    this.modalMessage = msg;
                    openDialog('success_modal');
                    setTimeout(() => location.reload(), 1500);
                })
                .catch(err => this.showError('Save Error', err.message));
            } catch (err) {
                this.showError('Validation Error', err.message);
            }
        },

        openImportModal() {
            this.importFile = null;
            this.importFileName = '';
            this.importMerge = true;
            this.importLoading = false;
            openDialog('import_modal');
        },

        openFeaturesModal() {
            openDialog('features_modal');
        },

        handleFileSelect(event) {
            const file = event.target.files[0];
            if (file) {
                this.importFile = file;
                this.importFileName = file.name;
            }
        },

        importOpenAPI() {
            if (!this.importFile) return;
            this.importLoading = true;
            const formData = new FormData();
            formData.append('file', this.importFile);
            formData.append('merge', this.importMerge ? 'true' : 'false');

            fetch('/import-openapi', { method: 'POST', body: formData })
                .then(res => res.json())
                .then(data => {
                    this.importLoading = false;
                    closeDialog('import_modal');
                    this.modalMessage = data.message;
                    openDialog('success_modal');
                    setTimeout(() => location.reload(), 1500);
                })
                .catch(err => {
                    this.importLoading = false;
                    this.showError('Import Failed', err.message);
                });
        },

        init() {
            let data = window.serverConfigs || [];
            this.configs = data.map(cfg => {
                const config = {
                    name: cfg.name || 'Unnamed',
                    method: cfg.method || 'GET',
                    path: cfg.path || '/',
                    statusCode: cfg.statusCode || 200,
                    timeout: cfg.timeout || 0,
                    requestHeaders: JSON.stringify(cfg.requestHeaders || {}, null, 2),
                    requestBody: JSON.stringify(cfg.requestBody || {}, null, 2),
                    responseHeaders: JSON.stringify(cfg.responseHeaders || {}, null, 2),
                    responseBody: JSON.stringify(cfg.responseBody || {}, null, 2),
                    responses: [],
                };

                if (cfg.responses) {
                    config.responses = cfg.responses.map(resp => ({
                        name: resp.name || 'Unnamed Response',
                        ruleOperator: resp.ruleOperator || 'AND',
                        rules: resp.rules || [],
                        response: {
                            statusCode: resp.response?.statusCode || 200,
                            timeout: resp.response?.timeout || 0,
                            headers: JSON.stringify(resp.response?.headers || {}, null, 2),
                            body: JSON.stringify(resp.response?.body || {}, null, 2)
                        }
                    }));
                }

                return config;
            });

            if (this.configs.length > 0) this.selectedIndex = 0;
        }
    }
}
