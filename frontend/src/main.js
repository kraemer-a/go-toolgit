// Check if running in Wails context
const isWails = window.go !== undefined;

// Import Wails runtime for opening external URLs
import { BrowserOpenURL } from '../wailsjs/runtime/runtime.js';

// Global function to open PR URLs
window.openPR = function(url) {
    if (isWails) {
        BrowserOpenURL(url);
    } else {
        // Fallback for demo mode
        window.open(url, '_blank');
    }
};

// Helper to get the app reference
const getApp = () => window.go?.gui?.App;

// Define backend functions
const GetConfig = isWails ? 
    async () => {
        console.log('Attempting GetConfig...');
        const app = getApp();
        if (app && app.GetConfig) {
            console.log('Found GetConfig on go.gui.App');
            return await app.GetConfig();
        }
        console.log('No GetConfig method found, using fallback');
        return {};
    } : 
    () => Promise.resolve({});

const UpdateConfig = isWails ? 
    async (config) => {
        console.log('UpdateConfig wrapper called with:', config);
        const app = getApp();
        console.log('getApp() returned:', app);
        
        if (!app) {
            console.error('No app reference available');
            throw new Error('Wails app not available');
        }
        
        if (app.UpdateConfig) {
            console.log('Found UpdateConfig method, calling it...');
            try {
                const result = await app.UpdateConfig(config);
                console.log('UpdateConfig result:', result);
                return result;
            } catch (err) {
                console.error('UpdateConfig call failed:', err);
                throw err;
            }
        } else {
            console.error('UpdateConfig method not found on app object');
            console.log('Available methods:', Object.keys(app));
            throw new Error('UpdateConfig method not available');
        }
    } : 
    (config) => {
        console.log('Demo mode: UpdateConfig called with:', config);
        return Promise.resolve();
    };

const ValidateAccess = isWails ? 
    async () => {
        const app = getApp();
        if (app && app.ValidateAccess) {
            return await app.ValidateAccess();
        }
        return;
    } : 
    () => Promise.resolve();

const ListRepositories = isWails ? 
    async () => {
        const app = getApp();
        if (app && app.ListRepositories) {
            return await app.ListRepositories();
        }
        return [];
    } : 
    () => Promise.resolve([]);

const ProcessReplacements = isWails ? 
    async (rules, repos, options) => {
        const app = getApp();
        if (app && app.ProcessReplacements) {
            return await app.ProcessReplacements(rules, repos, options);
        }
        return { success: true, message: 'Demo mode', repository_results: [] };
    } : 
    (rules, repos, options) => Promise.resolve({ success: true, message: 'Demo mode', repository_results: [] });

const SearchRepositories = isWails ? 
    async (criteria) => {
        const app = getApp();
        if (app && app.SearchRepositories) {
            return await app.SearchRepositories(criteria);
        }
        return [];
    } : 
    (criteria) => Promise.resolve([]);

const ProcessSearchReplacements = isWails ? 
    async (criteria, rules, options) => {
        const app = getApp();
        if (app && app.ProcessSearchReplacements) {
            return await app.ProcessSearchReplacements(criteria, rules, options);
        }
        return { success: true, message: 'Demo mode', repository_results: [] };
    } : 
    (criteria, rules, options) => Promise.resolve({ success: true, message: 'Demo mode', repository_results: [] });

const GetDefaultIncludePatterns = isWails ? 
    async () => {
        const app = getApp();
        if (app && app.GetDefaultIncludePatterns) {
            return await app.GetDefaultIncludePatterns();
        }
        return ['*.go', '*.js', '*.ts', '*.py'];
    } : 
    () => Promise.resolve(['*.go', '*.js', '*.ts', '*.py']);

const GetDefaultExcludePatterns = isWails ? 
    async () => {
        const app = getApp();
        if (app && app.GetDefaultExcludePatterns) {
            return await app.GetDefaultExcludePatterns();
        }
        return ['node_modules/*', 'vendor/*'];
    } : 
    () => Promise.resolve(['node_modules/*', 'vendor/*']);

class GitHubBitbucketReplaceApp {
    constructor() {
        this.repositories = [];
        this.replacementRules = [];
        this.config = {};
        this.init();
    }

    async init() {
        // Wait for Wails to be ready
        if (isWails) {
            await this.waitForWails();
        }
        
        // Debug: log what's available
        if (isWails) {
            console.log('Wails detected! Available methods:', window.go);
            try {
                console.log('window.go keys:', Object.keys(window.go));
                if (window.go.gui) {
                    console.log('window.go.gui keys:', Object.keys(window.go.gui));
                    if (window.go.gui.App) {
                        console.log('Found gui.App methods:', Object.keys(window.go.gui.App));
                    }
                }
            } catch (e) {
                console.log('Error inspecting window.go:', e);
            }
        } else {
            console.log('Running in demo mode - no Wails detected');
        }
        
        await this.loadConfig();
        this.setupEventListeners();
        this.renderConfigForm();
        this.setupProviderSwitching();
        this.addFloatingAnimation();
        this.addInteractiveEffects();
    }
    
    async waitForWails() {
        return new Promise((resolve) => {
            let attempts = 0;
            const checkWails = () => {
                attempts++;
                console.log(`Checking for Wails (attempt ${attempts})...`);
                
                if (window.go && window.go.gui && window.go.gui.App) {
                    console.log('Wails is ready with gui.App available');
                    resolve();
                } else if (attempts > 20) {
                    console.warn('Wails not fully loaded after 20 attempts, proceeding anyway');
                    resolve();
                } else {
                    console.log('Waiting for Wails to be ready...', {
                        'window.go': !!window.go,
                        'window.go.gui': !!(window.go && window.go.gui),
                        'window.go.gui.App': !!(window.go && window.go.gui && window.go.gui.App)
                    });
                    setTimeout(checkWails, 100);
                }
            };
            checkWails();
        });
    }

    async loadConfig() {
        try {
            this.config = await GetConfig();
            console.log('Loaded config:', this.config);
        } catch (error) {
            console.error('Failed to load config:', error);
            this.showError('Failed to load configuration');
        }
    }

    setupEventListeners() {
        // Handle update config button click
        document.getElementById('update-config-btn').addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.handleConfigSubmit(e);
        });
        
        document.getElementById('validate-btn').addEventListener('click', this.handleValidateAccess.bind(this));
        document.getElementById('list-repos-btn').addEventListener('click', this.handleListRepositories.bind(this));
        document.getElementById('add-rule-btn').addEventListener('click', this.handleAddRule.bind(this));
        document.getElementById('process-btn').addEventListener('click', this.handleProcessReplacements.bind(this));
        document.getElementById('dry-run-btn').addEventListener('click', () => this.handleProcessReplacements(true));
        document.getElementById('provider').addEventListener('change', this.handleProviderChange.bind(this));
        
        // Diff preview buttons
        document.getElementById('apply-changes-btn').addEventListener('click', this.handleApplyChanges.bind(this));
        document.getElementById('cancel-changes-btn').addEventListener('click', this.handleCancelChanges.bind(this));
    }

    renderConfigForm() {
        // Set provider
        document.getElementById('provider').value = this.config.provider || 'github';
        
        // GitHub fields
        document.getElementById('github-url').value = this.config.github_url || 'https://api.github.com';
        document.getElementById('token').value = this.config.token || '';
        document.getElementById('organization').value = this.config.organization || '';
        document.getElementById('team').value = this.config.team || '';
        
        // Bitbucket fields
        document.getElementById('bitbucket-url').value = this.config.bitbucket_url || '';
        document.getElementById('bitbucket-username').value = this.config.bitbucket_username || '';
        document.getElementById('bitbucket-password').value = this.config.bitbucket_password || '';
        document.getElementById('bitbucket-project').value = this.config.bitbucket_project || '';
        
        // Show appropriate config section
        this.updateProviderVisibility(this.config.provider || 'github');
    }

    async handleConfigSubmit(e) {
        console.log('handleConfigSubmit called');
        if (e) {
            e.preventDefault();
            e.stopPropagation();
        }
        
        // Get form values directly to avoid issues
        const config = {
            provider: document.getElementById('provider').value,
            github_url: document.getElementById('github-url').value,
            token: document.getElementById('token').value,
            organization: document.getElementById('organization').value,
            team: document.getElementById('team').value,
            bitbucket_url: document.getElementById('bitbucket-url').value,
            bitbucket_username: document.getElementById('bitbucket-username').value,
            bitbucket_password: document.getElementById('bitbucket-password').value,
            bitbucket_project: document.getElementById('bitbucket-project').value
        };

        console.log('Config to save:', config);

        try {
            // Test if we can access the App methods
            const app = getApp();
            console.log('App reference:', app);
            
            if (app && app.Greet) {
                console.log('Testing Greet method...');
                const greeting = await app.Greet("Test");
                console.log('Greet test result:', greeting);
            } else {
                console.log('Greet method not found on app');
            }
            
            console.log('Calling UpdateConfig...');
            await UpdateConfig(config);
            console.log('UpdateConfig returned successfully');
            
            this.config = config;
            this.showSuccess('Configuration updated successfully');
            
            // Explicitly enable the validate button
            const validateBtn = document.getElementById('validate-btn');
            if (validateBtn) {
                validateBtn.disabled = false;
                console.log('Validate button enabled directly');
            }
            
            console.log('Configuration update completed');
        } catch (error) {
            console.error('Failed to update config:', error);
            this.showError('Failed to update configuration: ' + (error.message || error));
        }
        
        return false; // Prevent any default action
    }

    async handleValidateAccess() {
        this.showProgress('Validating access...');
        
        try {
            await ValidateAccess();
            this.showSuccess('Access validation successful');
            document.getElementById('list-repos-btn').disabled = false;
        } catch (error) {
            console.error('Validation failed:', error);
            this.showError('Access validation failed: ' + error);
        }
    }

    async handleListRepositories() {
        this.showProgress('Loading repositories...');
        
        try {
            this.repositories = await ListRepositories();
            this.renderRepositories();
            this.showSuccess(`Found ${this.repositories.length} repositories`);
            this.enableReplacementSection();
        } catch (error) {
            console.error('Failed to list repositories:', error);
            this.showError('Failed to list repositories: ' + error);
        }
    }

    renderRepositories() {
        const container = document.getElementById('repositories-list');
        container.innerHTML = '';

        if (this.repositories.length === 0) {
            container.innerHTML = '<p>No repositories found</p>';
            return;
        }

        const selectAllDiv = document.createElement('div');
        selectAllDiv.className = 'select-all-container';
        selectAllDiv.innerHTML = `
            <label>
                <input type="checkbox" id="select-all-repos"> Select All
            </label>
        `;
        container.appendChild(selectAllDiv);

        document.getElementById('select-all-repos').addEventListener('change', (e) => {
            const checkboxes = container.querySelectorAll('input[type="checkbox"]:not(#select-all-repos)');
            checkboxes.forEach(cb => cb.checked = e.target.checked);
        });

        this.repositories.forEach((repo, index) => {
            const repoDiv = document.createElement('div');
            repoDiv.className = 'repository-item';
            repoDiv.innerHTML = `
                <label>
                    <input type="checkbox" value="${index}">
                    <span class="repo-name">${repo.full_name}</span>
                    <span class="repo-visibility ${repo.private ? 'private' : 'public'}">
                        ${repo.private ? 'Private' : 'Public'}
                    </span>
                </label>
            `;
            container.appendChild(repoDiv);
        });
    }

    handleAddRule() {
        const container = document.getElementById('replacement-rules');
        const ruleIndex = this.replacementRules.length;
        
        const ruleDiv = document.createElement('div');
        ruleDiv.className = 'rule-item';
        ruleDiv.style.opacity = '0';
        ruleDiv.style.transform = 'translateY(20px)';
        ruleDiv.innerHTML = `
            <div class="rule-inputs">
                <div class="form-group">
                    <label>Original String</label>
                    <input type="text" placeholder="Enter text to find..." class="rule-original" required>
                </div>
                <div class="form-group">
                    <label>Replacement String</label>
                    <input type="text" placeholder="Enter replacement text..." class="rule-replacement" required>
                </div>
                <label><input type="checkbox" class="rule-regex"> 🔍 Regex</label>
                <label><input type="checkbox" class="rule-case-sensitive" checked> 🔤 Case Sensitive</label>
                <label><input type="checkbox" class="rule-whole-word"> 📝 Whole Word</label>
                <button type="button" class="remove-rule-btn danger" onclick="window.removeRule(this)">🗑️ Remove</button>
            </div>
        `;
        
        container.appendChild(ruleDiv);
        
        // Animate the new rule in
        setTimeout(() => {
            ruleDiv.style.transition = 'all 0.3s ease';
            ruleDiv.style.opacity = '1';
            ruleDiv.style.transform = 'translateY(0)';
        }, 10);
        
        // Focus the first input
        setTimeout(() => {
            ruleDiv.querySelector('.rule-original').focus();
        }, 300);
    }

    async handleProcessReplacements(dryRun = false) {
        const rules = this.collectReplacementRules();
        const selectedRepos = this.collectSelectedRepositories();
        const options = this.collectProcessingOptions(dryRun);

        if (rules.length === 0) {
            this.showError('Please add at least one replacement rule');
            return;
        }

        if (selectedRepos.length === 0) {
            this.showError('Please select at least one repository');
            return;
        }

        this.showProgress(dryRun ? 'Running dry run...' : 'Processing replacements...');
        
        try {
            const result = await ProcessReplacements(rules, selectedRepos, options);
            
            if (dryRun && result.success && result.diffs) {
                // Store the result for later use
                this.pendingChanges = {
                    rules: rules,
                    selectedRepos: selectedRepos,
                    options: {...options, dry_run: false},
                    diffs: result.diffs
                };
                
                // Show diff preview
                this.showDiffPreview(result.diffs);
            } else {
                this.renderResults(result);
                
                if (result.success) {
                    this.showSuccess(result.message);
                } else {
                    this.showError(result.message);
                }
            }
        } catch (error) {
            console.error('Processing failed:', error);
            this.showError('Processing failed: ' + error);
        }
    }

    collectReplacementRules() {
        const rules = [];
        const ruleItems = document.querySelectorAll('.rule-item');
        
        ruleItems.forEach(item => {
            const original = item.querySelector('.rule-original').value.trim();
            const replacement = item.querySelector('.rule-replacement').value.trim();
            
            if (original) {
                rules.push({
                    original: original,
                    replacement: replacement,
                    regex: item.querySelector('.rule-regex').checked,
                    case_sensitive: item.querySelector('.rule-case-sensitive').checked,
                    whole_word: item.querySelector('.rule-whole-word').checked
                });
            }
        });
        
        return rules;
    }

    collectSelectedRepositories() {
        const selected = [];
        const checkboxes = document.querySelectorAll('#repositories-list input[type="checkbox"]:not(#select-all-repos)');
        
        checkboxes.forEach(cb => {
            if (cb.checked) {
                const index = parseInt(cb.value);
                const repo = {...this.repositories[index]};
                repo.selected = true;
                selected.push(repo);
            }
        });
        
        return selected;
    }

    collectProcessingOptions(dryRun) {
        return {
            include_patterns: this.getArrayFromTextarea('include-patterns'),
            exclude_patterns: this.getArrayFromTextarea('exclude-patterns'),
            dry_run: dryRun,
            pr_title: document.getElementById('pr-title').value || 'Automated string replacement',
            pr_body: document.getElementById('pr-body').value || 'Automated string replacement performed by GitHub Replace Tool',
            branch_prefix: document.getElementById('branch-prefix').value || 'auto-replace'
        };
    }

    getArrayFromTextarea(id) {
        const value = document.getElementById(id).value.trim();
        return value ? value.split('\n').map(s => s.trim()).filter(s => s.length > 0) : [];
    }

    renderResults(result) {
        const container = document.getElementById('results');
        container.innerHTML = '';

        if (!result.repository_results || result.repository_results.length === 0) {
            container.innerHTML = '<p>No results to display</p>';
            return;
        }

        const resultsDiv = document.createElement('div');
        resultsDiv.className = 'results-container';
        
        result.repository_results.forEach(repoResult => {
            const resultDiv = document.createElement('div');
            resultDiv.className = `result-item ${repoResult.success ? 'success' : 'error'}`;
            
            // Debug info to show what we received
            const debugInfo = `Debug: PR URL field = "${repoResult.pr_url || 'undefined'}"`;
            
            resultDiv.innerHTML = `
                <h4>${repoResult.repository}</h4>
                <p>${repoResult.message}</p>
                ${repoResult.pr_url ? 
                    `<p><button onclick="window.openPR('${repoResult.pr_url}')" style="background: #10b981; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer; text-decoration: none;">🔗 View Pull Request</button></p>` : 
                    '<p style="color: #ef4444;">❌ No PR URL available</p>'
                }
                <p>Files changed: ${repoResult.files_changed.length}, Replacements: ${repoResult.replacements}</p>
                <p style="font-size: 0.8em; color: #6b7280;">${debugInfo}</p>
            `;
            resultsDiv.appendChild(resultDiv);
        });

        container.appendChild(resultsDiv);
    }

    enableNextSteps() {
        console.log('enableNextSteps() called');
        const validateBtn = document.getElementById('validate-btn');
        console.log('Validate button element:', validateBtn);
        console.log('Current validate button disabled state:', validateBtn?.disabled);
        
        if (validateBtn) {
            validateBtn.disabled = false;
            console.log('Validate button enabled, new disabled state:', validateBtn.disabled);
            
            // Double-check by inspecting the actual DOM element
            setTimeout(() => {
                const checkBtn = document.getElementById('validate-btn');
                console.log('Post-timeout check - validate button disabled:', checkBtn?.disabled);
                console.log('Post-timeout check - validate button classes:', checkBtn?.className);
            }, 100);
        } else {
            console.error('Validate button not found!');
        }
    }

    enableReplacementSection() {
        document.getElementById('replacement-section').style.display = 'block';
    }

    setupProviderSwitching() {
        this.updateProviderVisibility(document.getElementById('provider').value);
    }

    handleProviderChange(e) {
        const provider = e.target.value;
        this.updateProviderVisibility(provider);
        
        // Reset status and disable next steps when provider changes
        const status = document.getElementById('status');
        status.style.display = 'none';
        document.getElementById('validate-btn').disabled = true;
        document.getElementById('list-repos-btn').disabled = true;
        document.getElementById('repositories-list').innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📂</div>
                <p>Configure and validate access first to see repositories.</p>
            </div>
        `;
        document.getElementById('replacement-section').style.display = 'none';
        document.getElementById('replacement-section').classList.remove('show');
        
        this.addFloatingAnimation();
    }

    updateProviderVisibility(provider) {
        const githubConfig = document.getElementById('github-config');
        const bitbucketConfig = document.getElementById('bitbucket-config');
        
        if (provider === 'github') {
            githubConfig.style.display = 'block';
            bitbucketConfig.style.display = 'none';
            
            // Set required attributes for GitHub fields (org/team are optional)
            document.getElementById('github-url').required = true;
            document.getElementById('token').required = true;
            document.getElementById('organization').required = false;
            document.getElementById('team').required = false;
            
            // Remove required attributes from Bitbucket fields
            document.getElementById('bitbucket-url').required = false;
            document.getElementById('bitbucket-username').required = false;
            document.getElementById('bitbucket-password').required = false;
            document.getElementById('bitbucket-project').required = false;
        } else if (provider === 'bitbucket') {
            githubConfig.style.display = 'none';
            bitbucketConfig.style.display = 'block';
            
            // Remove required attributes from GitHub fields
            document.getElementById('github-url').required = false;
            document.getElementById('token').required = false;
            document.getElementById('organization').required = false;
            document.getElementById('team').required = false;
            
            // Set required attributes for Bitbucket fields
            document.getElementById('bitbucket-url').required = true;
            document.getElementById('bitbucket-username').required = true;
            document.getElementById('bitbucket-password').required = true;
            document.getElementById('bitbucket-project').required = true;
        }
    }

    showProgress(message) {
        const status = document.getElementById('status');
        status.className = 'status progress';
        status.textContent = message;
    }

    showSuccess(message) {
        const status = document.getElementById('status');
        if (status) {
            status.className = 'status success';
            status.textContent = message;
            status.style.display = 'flex';
            console.log('Success message displayed:', message);
        } else {
            console.error('Status element not found!');
        }
    }

    showError(message) {
        const status = document.getElementById('status');
        if (status) {
            status.className = 'status error';
            status.textContent = message;
            status.style.display = 'flex';
            console.error('Error message displayed:', message);
        } else {
            console.error('Status element not found!');
        }
    }
    
    addLoadingAnimation() {
        const sections = document.querySelectorAll('.section');
        sections.forEach(section => {
            section.classList.add('loading');
        });
    }
    
    removeLoadingAnimation() {
        const sections = document.querySelectorAll('.section');
        sections.forEach(section => {
            section.classList.remove('loading');
        });
    }
    
    animateSuccess() {
        const status = document.getElementById('status');
        status.style.animation = 'none';
        setTimeout(() => {
            status.style.animation = 'slideIn 0.3s ease-out';
        }, 10);
    }
    
    animateError() {
        const status = document.getElementById('status');
        status.style.animation = 'none';
        setTimeout(() => {
            status.style.animation = 'slideIn 0.3s ease-out';
        }, 10);
    }
    
    addFloatingAnimation() {
        const icons = document.querySelectorAll('.empty-icon');
        icons.forEach(icon => {
            icon.classList.add('floating');
        });
    }
    
    showDiffPreview(diffs) {
        const diffSection = document.getElementById('diff-preview-section');
        const diffContent = document.getElementById('diff-content');
        
        // Show the section
        diffSection.style.display = 'block';
        
        // Calculate stats
        let totalFiles = 0;
        let totalAdditions = 0;
        let totalDeletions = 0;
        
        // Clear previous content
        diffContent.innerHTML = '';
        
        // Render each repository's diffs
        Object.entries(diffs).forEach(([repoName, repoDiffs]) => {
            const repoDiv = document.createElement('div');
            repoDiv.className = 'diff-repository';
            repoDiv.innerHTML = `<h3 style="margin: 1rem 0; color: var(--primary-color);">📁 ${repoName}</h3>`;
            
            Object.entries(repoDiffs).forEach(([fileName, fileDiff]) => {
                totalFiles++;
                const fileDiv = this.createDiffFileElement(fileName, fileDiff);
                repoDiv.appendChild(fileDiv);
                
                // Count additions and deletions
                const lines = fileDiff.split('\n');
                lines.forEach(line => {
                    if (line.startsWith('+') && !line.startsWith('+++')) totalAdditions++;
                    if (line.startsWith('-') && !line.startsWith('---')) totalDeletions++;
                });
            });
            
            diffContent.appendChild(repoDiv);
        });
        
        // Update stats
        document.getElementById('diff-files-count').textContent = `${totalFiles} file${totalFiles !== 1 ? 's' : ''}`;
        document.getElementById('diff-additions').textContent = `+${totalAdditions}`;
        document.getElementById('diff-deletions').textContent = `-${totalDeletions}`;
        
        // Scroll to diff section
        diffSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    
    createDiffFileElement(fileName, diffContent) {
        const fileDiv = document.createElement('div');
        fileDiv.className = 'diff-file';
        
        const headerDiv = document.createElement('div');
        headerDiv.className = 'diff-file-header';
        headerDiv.textContent = fileName;
        fileDiv.appendChild(headerDiv);
        
        const contentDiv = document.createElement('div');
        contentDiv.className = 'diff-file-content';
        
        const lines = diffContent.split('\n');
        let lineNumber = 0;
        
        lines.forEach((line, index) => {
            const lineDiv = document.createElement('div');
            lineDiv.className = 'diff-line';
            
            // Determine line type
            let lineClass = 'diff-line-context';
            if (line.startsWith('@@')) {
                lineClass = 'diff-hunk-header';
            } else if (line.startsWith('+') && !line.startsWith('+++')) {
                lineClass = 'diff-line-addition';
                lineNumber++;
            } else if (line.startsWith('-') && !line.startsWith('---')) {
                lineClass = 'diff-line-deletion';
            } else if (!line.startsWith('\\') && !line.startsWith('+++') && !line.startsWith('---')) {
                lineNumber++;
            }
            
            lineDiv.className += ' ' + lineClass;
            
            // Add line number
            const lineNumDiv = document.createElement('div');
            lineNumDiv.className = 'diff-line-number';
            if (lineClass === 'diff-line-addition' || lineClass === 'diff-line-context') {
                lineNumDiv.textContent = lineNumber.toString();
            }
            lineDiv.appendChild(lineNumDiv);
            
            // Add line content
            const lineContentDiv = document.createElement('div');
            lineContentDiv.className = 'diff-line-content';
            lineContentDiv.textContent = line || ' ';
            lineDiv.appendChild(lineContentDiv);
            
            contentDiv.appendChild(lineDiv);
        });
        
        fileDiv.appendChild(contentDiv);
        return fileDiv;
    }
    
    async handleApplyChanges() {
        if (!this.pendingChanges) {
            this.showError('No pending changes to apply');
            return;
        }
        
        this.showProgress('Applying changes and creating pull requests...');
        
        try {
            const result = await ProcessReplacements(
                this.pendingChanges.rules,
                this.pendingChanges.selectedRepos,
                this.pendingChanges.options
            );
            
            this.renderResults(result);
            
            if (result.success) {
                this.showSuccess(result.message);
                this.hideDiffPreview();
            } else {
                this.showError(result.message);
            }
        } catch (error) {
            console.error('Failed to apply changes:', error);
            this.showError('Failed to apply changes: ' + error);
        }
    }
    
    handleCancelChanges() {
        this.hideDiffPreview();
        this.showSuccess('Changes cancelled');
    }
    
    hideDiffPreview() {
        document.getElementById('diff-preview-section').style.display = 'none';
        this.pendingChanges = null;
    }
    
    addInteractiveEffects() {
        // Add hover effects to sections
        const sections = document.querySelectorAll('.section');
        sections.forEach(section => {
            section.addEventListener('mouseenter', () => {
                section.style.transform = 'translateY(-2px)';
            });
            section.addEventListener('mouseleave', () => {
                section.style.transform = 'translateY(0)';
            });
        });
        
        // Add ripple effect to buttons
        const buttons = document.querySelectorAll('button');
        buttons.forEach(button => {
            button.addEventListener('click', function(e) {
                const ripple = document.createElement('span');
                const rect = this.getBoundingClientRect();
                const size = Math.max(rect.width, rect.height);
                const x = e.clientX - rect.left - size / 2;
                const y = e.clientY - rect.top - size / 2;
                
                ripple.style.cssText = `
                    position: absolute;
                    width: ${size}px;
                    height: ${size}px;
                    left: ${x}px;
                    top: ${y}px;
                    background: rgba(255, 255, 255, 0.3);
                    border-radius: 50%;
                    transform: scale(0);
                    animation: ripple 0.6s ease-out;
                    pointer-events: none;
                `;
                
                this.appendChild(ripple);
                
                setTimeout(() => {
                    ripple.remove();
                }, 600);
            });
        });
        
        // Add custom scrollbar styling
        const style = document.createElement('style');
        style.textContent = `
            @keyframes ripple {
                to {
                    transform: scale(2);
                    opacity: 0;
                }
            }
        `;
        document.head.appendChild(style);
    }
}

// Global function for removing rules
window.removeRule = function(button) {
    const ruleItem = button.closest('.rule-item');
    ruleItem.style.transition = 'all 0.3s ease';
    ruleItem.style.opacity = '0';
    ruleItem.style.transform = 'translateX(-20px)';
    setTimeout(() => {
        ruleItem.remove();
    }, 300);
};

// Add page load tracking
window.addEventListener('load', () => {
    console.log('=== Page loaded at', new Date().toISOString());
});

window.addEventListener('beforeunload', (e) => {
    console.log('=== Page is unloading!');
});

// Initialize the app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    console.log('=== DOMContentLoaded, initializing app');
    new GitHubBitbucketReplaceApp();
});