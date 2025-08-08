/**
 * Oar Streaming System - Consolidated
 *
 * Handles EventSource streaming for deploy, stop, and logs operations.
 * Manages project state updates and UI synchronization.
 */

class OarStreaming {
    constructor() {
        this.projectStates = {};
        this.eventSources = {};
        this.initializeModalListeners();
    }

    // ===== CORE STREAMING FUNCTIONALITY =====

    /**
     * Start a streaming operation
     */
    startStream(config) {
        const { projectId, operation, url, button } = config;

        // Validate config
        if (!projectId || !operation || !url) {
            console.error('Missing required streaming parameters');
            return null;
        }

        const streamKey = `${projectId}-${operation}`;

        // Close existing stream if running
        this.stopStream(streamKey);

        // Get DOM elements
        const outputContent = document.getElementById(`${operation}-output-content-${projectId}`);
        if (!outputContent) {
            console.error(`Output element not found: ${operation}-output-content-${projectId}`);
            return null;
        }

        // Initialize output
        const initialMessage = this.getInitialMessage(operation);
        outputContent.innerHTML = `<span class="oar-text">${initialMessage}</span>\n\n`;
        outputContent.lastWasDockerCompose = false;

        // Set button state
        if (button) {
            this.setButtonBusy(button, this.getButtonText(operation, 'loading'));
        }

        // Create EventSource
        const eventSource = new EventSource(url);
        this.eventSources[streamKey] = eventSource;

        // Handle messages
        eventSource.onmessage = (event) => {
            this.handleMessage(event, { projectId, operation, outputContent, button, eventSource });
        };

        // Handle errors
        eventSource.onerror = (event) => {
            this.handleError(event, { projectId, operation, outputContent, button, eventSource });
        };

        // Setup modal cleanup for logs
        if (operation === 'logs') {
            this.setupLogsCleanup(projectId, eventSource);
        }

        return eventSource;
    }

    /**
     * Stop a streaming operation
     */
    stopStream(streamKey) {
        const eventSource = this.eventSources[streamKey];
        if (eventSource) {
            eventSource.close();
            delete this.eventSources[streamKey];
        }
    }

    /**
     * Handle streaming messages
     */
    handleMessage(event, context) {
        const { projectId, operation, outputContent, button, eventSource } = context;

        try {
            const message = JSON.parse(event.data);

            // Store project state updates
            if (message.projectState) {
                this.projectStates[projectId] = message.projectState;
            }

            // Display message
            if (message.message) {
                if (message.type === 'info' && message.source === 'docker') {
                    this.addDockerOutput(outputContent, message.message);
                } else {
                    this.addOarOutput(outputContent, message.message, message.type);
                }
            }

            // Handle completion
            if ((message.type === 'success' || message.type === 'error') && message.projectState) {
                eventSource.close();

                if (button) {
                    this.setButtonComplete(button, projectId, operation);
                }
            }
        } catch (e) {
            // Fallback for non-JSON messages
            outputContent.innerHTML += event.data + '\n';
        }

        this.autoScroll(outputContent);
    }

    /**
     * Handle streaming errors
     */
    handleError(event, context) {
        const { projectId, operation, outputContent, button, eventSource } = context;

        const errorMessage = operation === 'logs'
            ? 'Connection error or log stream ended'
            : 'Connection error or process completed';

        this.addOarOutput(outputContent, errorMessage, 'error');
        eventSource.close();

        if (button) {
            this.setButtonComplete(button, projectId, operation);
        }
    }

    // ===== UI MANAGEMENT =====

    /**
     * Add Docker Compose output
     */
    addDockerOutput(outputContent, text) {
        const needsSeparation = outputContent.innerHTML.trim().length > 0 && !outputContent.lastWasDockerCompose;
        const prefix = needsSeparation ? '\n' : '';
        outputContent.innerHTML += prefix + text + '\n';
        outputContent.lastWasDockerCompose = true;
    }

    /**
     * Add Oar system output
     */
    addOarOutput(outputContent, text, type = '') {
        const needsSeparation = outputContent.lastWasDockerCompose;
        const prefix = needsSeparation ? '\n' : '';
        const className = type ? `oar-text ${type}` : 'oar-text';
        outputContent.innerHTML += `${prefix}<span class="${className}" style="font-style: italic;">${text}</span>\n`;
        outputContent.lastWasDockerCompose = false;
    }

    /**
     * Auto-scroll output to bottom
     */
    autoScroll(outputContent) {
        const codeElement = outputContent.closest('code');
        if (codeElement) {
            codeElement.scrollTop = codeElement.scrollHeight;
        }
    }

    /**
     * Set button to busy state
     */
    setButtonBusy(button, text) {
        button.disabled = true;
        button.textContent = text;
    }

    /**
     * Set button to complete state
     */
    setButtonComplete(button, projectId, operation) {
        button.disabled = false;
        button.textContent = 'Close';
        button.onclick = () => {
            this.closeModal(`${operation}-project-modal-${projectId}`);
            this.updateProjectCard(projectId);
        };
    }

    /**
     * Close modal
     */
    closeModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            const closeButton = modal.querySelector('[data-modal-close]');
            if (closeButton) {
                closeButton.click();
            } else {
                modal.setAttribute('data-open', 'false');
                modal.classList.add('hidden');
            }
        }
    }

    /**
     * Update project card with stored state
     */
    updateProjectCard(projectId) {
        setTimeout(() => {
            const projectState = this.projectStates[projectId];
            if (projectState) {
                this.updateProjectStatus(projectId, projectState.status);
                if (projectState.lastCommit) {
                    this.updateProjectCommit(projectId, projectState.lastCommit);
                }
                this.updateActionButtons(projectId, projectState.status);
                delete this.projectStates[projectId];
            }
        }, 100);
    }

    /**
     * Update project status badge
     */
    updateProjectStatus(projectId, status) {
        const statusBadge = document.querySelector(`#project-card-${projectId} .status-badge`);
        if (statusBadge) {
            // Remove all status classes
            statusBadge.classList.remove(
                'bg-green-200', 'text-green-800',
                'bg-gray-200', 'text-gray-800',
                'bg-red-200', 'text-red-800',
                'bg-yellow-200', 'text-yellow-800'
            );

            // Add new status classes
            const statusClasses = this.getStatusClasses(status);
            statusBadge.classList.add(...statusClasses);
            statusBadge.textContent = status;
            statusBadge.setAttribute('data-status', status);
        }
    }

    /**
     * Update project commit badge
     */
    updateProjectCommit(projectId, commit) {
        const commitBadge = document.querySelector(`#project-card-${projectId} .commit-badge`);
        if (commitBadge) {
            commitBadge.classList.remove('bg-gray-100', 'text-gray-500');
            commitBadge.classList.add('bg-gray-200', 'text-gray-800');
            commitBadge.textContent = commit.substring(0, 8);
            commitBadge.setAttribute('data-commit', commit);
        }
    }

    /**
     * Update action buttons based on status
     */
    updateActionButtons(projectId, status) {
        const projectCard = document.getElementById(`project-card-${projectId}`);
        if (!projectCard) return;

        const isRunning = status === 'running';

        // Update stop button visibility
        const stopButton = projectCard.querySelector('button.text-red-600');
        if (stopButton) {
            const container = stopButton.closest('.relative.group')?.parentElement;
            if (container && container.classList.contains('hidden') !== undefined) {
                container.classList.toggle('hidden', !isRunning);
            }
        }

        // Update edit button state
        const editButton = projectCard.querySelector('[data-modal-trigger*="edit"]');
        if (editButton) {
            editButton.disabled = isRunning;
            editButton.classList.toggle('cursor-not-allowed', isRunning);
            editButton.classList.toggle('opacity-50', isRunning);

            if (isRunning) {
                editButton.removeAttribute('data-modal-trigger');
                editButton.title = 'Cannot edit running project - stop it first';
            } else {
                editButton.setAttribute('data-modal-trigger', `edit-project-modal-${projectId}`);
                editButton.title = 'Edit Project';
            }
        }
    }

    // ===== MODAL MANAGEMENT =====

    /**
     * Initialize modal event listeners
     */
    initializeModalListeners() {
        document.addEventListener('click', (event) => {
            const trigger = event.target.closest('[data-modal-trigger]');
            if (!trigger) return;

            const modalId = trigger.getAttribute('data-modal-trigger');
            const modalInfo = this.parseModalId(modalId);

            if (modalInfo) {
                setTimeout(() => {
                    this.resetModalContent(modalInfo.projectId, modalInfo.type);
                    if (modalInfo.type === 'logs') {
                        this.startLogsStream(modalInfo.projectId);
                    } else if (modalInfo.type === 'config') {
                        this.loadConfig(modalInfo.projectId);
                    }
                }, 100);
            }
        });
    }

    /**
     * Parse modal ID to extract project ID and type
     */
    parseModalId(modalId) {
        const patterns = [
            { pattern: 'deploy-project-modal-', type: 'deploy' },
            { pattern: 'stop-project-modal-', type: 'stop' },
            { pattern: 'logs-modal-', type: 'logs' },
            { pattern: 'config-modal-', type: 'config' }
        ];

        for (const { pattern, type } of patterns) {
            if (modalId.includes(pattern)) {
                return {
                    projectId: modalId.replace(pattern, ''),
                    type: type
                };
            }
        }
        return null;
    }

    /**
     * Reset modal content to initial state
     */
    resetModalContent(projectId, type) {
        const outputContent = document.getElementById(`${type}-output-content-${projectId}`);
        const button = document.getElementById(`${type}-button-${projectId}`);

        if (outputContent) {
            outputContent.innerHTML = `<span class="oar-text">${this.getInitialMessage(type)}</span>`;
            outputContent.lastWasDockerCompose = false;
        }

        if (button && type !== 'logs' && type !== 'config') {
            button.disabled = false;
            button.textContent = this.getButtonText(type, 'initial');
            button.onclick = () => {
                if (type === 'deploy') this.deployProject(projectId);
                if (type === 'stop') this.stopProject(projectId);
            };
        }
    }

    /**
     * Setup cleanup for logs modal
     */
    setupLogsCleanup(projectId, eventSource) {
        const modal = document.getElementById(`logs-modal-${projectId}`);
        if (!modal) return;

        const cleanup = () => {
            if (eventSource.readyState !== EventSource.CLOSED) {
                eventSource.close();
            }
        };

        // Listen for modal close events
        modal.addEventListener('modal:closed', cleanup, { once: true });

        // Listen for close button clicks
        const closeButtons = modal.querySelectorAll('[data-modal-close]');
        closeButtons.forEach(button => {
            button.addEventListener('click', cleanup, { once: true });
        });
    }

    // ===== OPERATION HANDLERS =====

    /**
     * Deploy project
     */
    deployProject(projectId) {
        const pullCheckbox = document.getElementById(`pull-changes-${projectId}`);
        const pullChanges = pullCheckbox ? pullCheckbox.checked : true;
        const url = `/projects/${projectId}/deploy/stream${pullChanges ? '?pull=true' : ''}`;
        const button = document.getElementById(`deploy-button-${projectId}`);

        return this.startStream({
            projectId,
            operation: 'deploy',
            url,
            button
        });
    }

    /**
     * Stop project
     */
    stopProject(projectId) {
        const url = `/projects/${projectId}/stop/stream`;
        const button = document.getElementById(`stop-button-${projectId}`);

        return this.startStream({
            projectId,
            operation: 'stop',
            url,
            button
        });
    }

    /**
     * Start logs stream
     */
    startLogsStream(projectId) {
        const url = `/projects/${projectId}/logs/stream`;

        return this.startStream({
            projectId,
            operation: 'logs',
            url
        });
    }

    /**
     * Load project configuration
     */
    loadConfig(projectId) {
        const outputContent = document.getElementById(`config-output-content-${projectId}`);
        if (!outputContent) return;

        fetch(`/projects/${projectId}/config`)
            .then(response => {
                if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                return response.text();
            })
            .then(configText => {
                outputContent.innerHTML = `<pre class="whitespace-pre-wrap text-gray-400">${this.escapeHtml(configText)}</pre>`;
            })
            .catch(error => {
                console.error('Error loading config:', error);
                outputContent.innerHTML = '<span class="oar-text text-red-400">Failed to load configuration. Please try again.</span>';
            });
    }

    // ===== UTILITY FUNCTIONS =====

    getInitialMessage(operation) {
        const messages = {
            deploy: 'Deploying project...',
            stop: 'Stopping project...',
            logs: 'Starting log stream...',
            config: ''
        };
        return messages[operation] || '';
    }

    getButtonText(operation, state) {
        const texts = {
            deploy: { initial: 'Deploy Project', loading: 'Deploying...' },
            stop: { initial: 'Stop Project', loading: 'Stopping...' },
            logs: { initial: null, loading: null },
            config: { initial: null, loading: null }
        };
        return texts[operation]?.[state];
    }

    getStatusClasses(status) {
        const classes = {
            running: ['bg-green-200', 'text-green-800'],
            stopped: ['bg-gray-200', 'text-gray-800'],
            error: ['bg-red-200', 'text-red-800'],
            unknown: ['bg-yellow-200', 'text-yellow-800']
        };
        return classes[status] || classes.unknown;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// ===== GLOBAL INTERFACE =====

// Create global instance
const oarStreaming = new OarStreaming();

// Export global functions for backward compatibility
window.deployProjectWithStreaming = (projectId) => oarStreaming.deployProject(projectId);
window.stopProjectWithStreaming = (projectId) => oarStreaming.stopProject(projectId);
window.viewLogsWithStreaming = (projectId) => oarStreaming.startLogsStream(projectId);
window.loadProjectConfig = (projectId) => oarStreaming.loadConfig(projectId);

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    console.log('Oar streaming system initialized');
});
