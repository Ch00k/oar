// Utility functions
function autoScroll(outputContent) {
    const codeElement = outputContent.closest('code');
    if (codeElement) {
        codeElement.scrollTop = codeElement.scrollHeight;
    }
}

function setupButtonCloseState(button) {
    button.textContent = 'Close';
    button.disabled = false;
    button.onclick = function () {
        window.location.reload();
    };
}

function addOarText(outputContent, text, type = '') {
    const className = type ? `oar-text ${type}` : 'oar-text';
    outputContent.innerHTML += `\n<span class="${className}">${text}</span>\n`;
}

// Generic streaming handler
function createStreamingHandler(config) {
    const {
        projectId,
        streamType,
        url,
        initialMessage,
        button,
        buttonDisabledText,
        onComplete,
        setupCleanup,
    } = config;

    const outputContent = document.getElementById(
        `${streamType}-output-content-${projectId}`,
    );
    if (!outputContent) {
        console.error(
            `Output content element not found for ${streamType} streaming`,
        );
        return;
    }

    // Clear and show initial message
    outputContent.innerHTML = `<span class="oar-text">${initialMessage}</span>\n\n`;

    // Disable button during operation
    if (button) {
        button.disabled = true;
        button.textContent = buttonDisabledText;
    }

    // Create EventSource connection
    const eventSource = new EventSource(url);

    // Setup cleanup if provided (for logs modal)
    if (setupCleanup) {
        setupCleanup(eventSource);
    }

    // Handle streaming messages
    eventSource.onmessage = function (event) {
        const data = event.data;

        // Check if this is a semantic OAR message
        if (data.startsWith('OAR_MSG:')) {
            const parts = data.split(':');
            if (parts.length >= 3) {
                const type = parts[1]; // default, success, error
                const message = parts.slice(2).join(':'); // rejoin in case message contains colons
                const cssType = type === 'default' ? '' : type; // use empty string for default styling
                addOarText(outputContent, message, cssType);
            } else {
                outputContent.innerHTML += data + '\n';
            }
        } else {
            outputContent.innerHTML += data + '\n';
        }

        autoScroll(outputContent);
    };

    // Handle successful completion
    eventSource.addEventListener('complete-success', function (event) {
        addOarText(outputContent, event.data, 'success');
        eventSource.close();

        if (button) {
            setupButtonCloseState(button);
        }

        autoScroll(outputContent);

        if (onComplete) {
            onComplete('success', event.data);
        }
    });

    // Handle error completion
    eventSource.addEventListener('complete-error', function (event) {
        addOarText(outputContent, event.data, 'error');
        eventSource.close();

        if (button) {
            setupButtonCloseState(button);
        }

        autoScroll(outputContent);

        if (onComplete) {
            onComplete('error', event.data);
        }
    });

    // Handle connection errors
    eventSource.onerror = function (event) {
        const errorMessage =
            streamType === 'logs'
                ? 'Connection error or log stream ended'
                : 'Connection error or process completed';

        addOarText(outputContent, errorMessage, 'error');
        eventSource.close();

        if (button) {
            setupButtonCloseState(button);
        }

        if (onComplete) {
            onComplete('error', errorMessage);
        }
    };

    return eventSource;
}

// Specific streaming functions
function deployProjectWithStreaming(projectId) {
    const deployButton = document.getElementById(`deploy-button-${projectId}`);
    const pullCheckbox = document.getElementById(`pull-changes-${projectId}`);

    // Get checkbox value for pull changes
    const pullChanges = pullCheckbox ? pullCheckbox.checked : true;
    const url = `/projects/${projectId}/deploy/stream${pullChanges ? '?pull=true' : ''}`;

    createStreamingHandler({
        projectId,
        streamType: 'deploy',
        url,
        initialMessage: 'Deploying project...',
        button: deployButton,
        buttonDisabledText: 'Deploying...',
    });
}

function stopProjectWithStreaming(projectId) {
    const stopButton = document.getElementById(`stop-button-${projectId}`);
    const url = `/projects/${projectId}/stop/stream`;

    createStreamingHandler({
        projectId,
        streamType: 'stop',
        url,
        initialMessage: 'Stopping project...',
        button: stopButton,
        buttonDisabledText: 'Stopping...',
    });
}

function viewLogsWithStreaming(projectId) {
    const modal = document.getElementById(`logs-modal-${projectId}`);
    const url = `/projects/${projectId}/logs/stream`;

    if (!modal) {
        console.error('Modal element not found for logs streaming');
        return;
    }

    createStreamingHandler({
        projectId,
        streamType: 'logs',
        url,
        initialMessage: 'Starting log stream...',
        setupCleanup: function (eventSource) {
            // Store eventSource reference on the modal for cleanup
            modal.logsEventSource = eventSource;

            // Add event listener for modal close to stop streaming
            const handleModalClose = function () {
                if (
                    eventSource &&
                    eventSource.readyState !== EventSource.CLOSED
                ) {
                    eventSource.close();
                }
            };

            // Listen for modal close events - try multiple possible event names
            modal.addEventListener('modal:closed', handleModalClose, {
                once: true,
            });
            modal.addEventListener('close', handleModalClose, { once: true });
            modal.addEventListener('hidden', handleModalClose, { once: true });

            // Also listen for close button clicks directly
            const closeButtons = modal.querySelectorAll('[data-modal-close]');
            closeButtons.forEach((button) => {
                button.addEventListener('click', handleModalClose, {
                    once: true,
                });
            });
        },
    });
}

// Modal content reset configuration
const modalConfigs = {
    deploy: {
        outputMessage:
            'Output will show up here when you deploy the project...',
        buttonText: 'Deploy Project',
        buttonHandler: deployProjectWithStreaming,
    },
    stop: {
        outputMessage: 'Output will show up here when you stop the project...',
        buttonText: 'Stop Project',
        buttonHandler: stopProjectWithStreaming,
    },
    logs: {
        outputMessage: 'Starting log stream...',
        buttonText: null,
        buttonHandler: null,
    },
};

// Reset modal content to fresh state
function resetModalContent(projectId, modalType) {
    const config = modalConfigs[modalType];
    if (!config) return;

    const outputContent = document.getElementById(
        `${modalType}-output-content-${projectId}`,
    );
    const button = document.getElementById(`${modalType}-button-${projectId}`);

    if (outputContent) {
        outputContent.innerHTML = `<span class="oar-text">${config.outputMessage}</span>`;
    }

    if (button && config.buttonText) {
        button.disabled = false;
        button.textContent = config.buttonText;
        button.onclick = function () {
            config.buttonHandler(projectId);
        };
    }
}

// Modal trigger patterns
const modalPatterns = [
    { pattern: 'deploy-project-modal-', type: 'deploy' },
    { pattern: 'stop-project-modal-', type: 'stop' },
    { pattern: 'logs-modal-', type: 'logs' },
];

// Extract project ID and modal type from modal ID
function parseModalId(modalId) {
    for (const { pattern, type } of modalPatterns) {
        if (modalId.includes(pattern)) {
            return {
                projectId: modalId.replace(pattern, ''),
                modalType: type,
            };
        }
    }
    return null;
}

// Handle modal opening and auto-start for logs
document.addEventListener('DOMContentLoaded', function () {
    // Listen for clicks on modal triggers
    document.addEventListener('click', function (event) {
        const trigger = event.target.closest('[data-modal-trigger]');
        if (!trigger) return;

        const modalId = trigger.getAttribute('data-modal-trigger');
        const parsed = parseModalId(modalId);

        if (parsed) {
            const { projectId, modalType } = parsed;

            // Reset modal content to fresh state
            setTimeout(function () {
                resetModalContent(projectId, modalType);

                // Auto-start logs streaming for logs modal
                if (modalType === 'logs') {
                    viewLogsWithStreaming(projectId);
                }
            }, 100); // Give modal time to open
        }
    });
});

