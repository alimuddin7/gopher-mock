function logViewer() {
    return {
        isOpen: false,
        isLive: true,
        autoScroll: true,
        logs: [],
        selected: null,
        detailTab: 'Response',
        filter: { query: '', method: 'ALL', status: 'all' },
        _evtSource: null,

        init() {
            // pre-load existing logs on first open
        },

        open() {
            this.isOpen = true;
            this._ensureSSE();
            if (this.logs.length === 0) {
                this._fetchAll();
            }
        },

        close() {
            this.isOpen = false;
        },

        toggleLive() {
            this.isLive = !this.isLive;
            if (this.isLive) {
                this._ensureSSE();
            } else {
                this._closeSSE();
            }
        },

        // ── SSE ─────────────────────────────────────────────────────
        _ensureSSE() {
            if (this._evtSource) return;
            const es = new EventSource('/api/logs/stream');
            es.onmessage = (e) => {
                const entry = JSON.parse(e.data);
                this.logs.push(entry);
                if (this.autoScroll) {
                    this.$nextTick(() => {
                        const el = document.getElementById('log-bottom');
                        if (el) el.scrollIntoView({ behavior: 'smooth' });
                    });
                }
            };
            es.onerror = () => {
                // reconnect is handled automatically by EventSource
            };
            this._evtSource = es;
        },

        _closeSSE() {
            if (this._evtSource) {
                this._evtSource.close();
                this._evtSource = null;
            }
        },

        // ── REST ────────────────────────────────────────────────────
        _fetchAll() {
            fetch('/api/logs')
                .then(r => r.json())
                .then(data => {
                    this.logs = data || [];
                    if (this.autoScroll) {
                        this.$nextTick(() => {
                            const el = document.getElementById('log-bottom');
                            if (el) el.scrollIntoView({ behavior: 'instant' });
                        });
                    }
                })
                .catch(() => {});
        },

        clearLogs() {
            fetch('/api/logs', { method: 'DELETE' })
                .then(() => {
                    this.logs = [];
                    this.selected = null;
                })
                .catch(() => {});
        },

        // ── Computed ─────────────────────────────────────────────────
        get filteredLogs() {
            return this.logs.filter(log => {
                const q = this.filter.query.toLowerCase();
                const matchQ = !q ||
                    log.url.toLowerCase().includes(q) ||
                    log.method.toLowerCase().includes(q);
                const matchM = this.filter.method === 'ALL' || log.method === this.filter.method;
                const matchS = this.filter.status === 'all' ||
                    String(log.statusCode).startsWith(this.filter.status);
                return matchQ && matchM && matchS;
            });
        },

        get successRate() {
            if (this.logs.length === 0) return 100;
            const ok = this.logs.filter(l => l.statusCode >= 200 && l.statusCode < 300).length;
            return Math.round((ok / this.logs.length) * 100);
        },

        get avgDuration() {
            if (this.logs.length === 0) return '0.0';
            const sum = this.logs.reduce((a, l) => a + l.durationMs, 0);
            return (sum / this.logs.length).toFixed(1);
        },

        // ── Actions ─────────────────────────────────────────────────
        select(log) {
            this.selected = log;
        },

        // ── Helpers ─────────────────────────────────────────────────
        methodClass(method) {
            return {
                'text-blue-500 bg-blue-500/10 border-blue-500/20':   method === 'GET',
                'text-emerald-500 bg-emerald-500/10 border-emerald-500/20': method === 'POST',
                'text-amber-500 bg-amber-500/10 border-amber-500/20':  method === 'PUT',
                'text-red-500 bg-red-500/10 border-red-500/20':       method === 'DELETE',
                'text-violet-500 bg-violet-500/10 border-violet-500/20': method === 'PATCH',
                'text-muted-foreground bg-muted border-border': !['GET','POST','PUT','DELETE','PATCH'].includes(method)
            };
        },

        statusClass(code) {
            if (code >= 500) return 'bg-red-500/15 text-red-500';
            if (code >= 400) return 'bg-amber-500/15 text-amber-500';
            if (code >= 300) return 'bg-blue-500/15 text-blue-500';
            return 'bg-emerald-500/15 text-emerald-500';
        },

        formatTime(ts) {
            const d = new Date(ts);
            return d.toLocaleTimeString('en-GB', { hour12: false }) +
                '.' + String(d.getMilliseconds()).padStart(3, '0');
        },

        formatJSON(val) {
            if (val === null || val === undefined) return '(empty)';
            if (typeof val === 'string') return val;
            try { return JSON.stringify(val, null, 2); }
            catch { return String(val); }
        }
    };
}
