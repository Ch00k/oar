// Utility functions
function autoScroll(outputContent) {
    const codeElement = outputContent.closest('code');
    if (codeElement) {
        codeElement.scrollTop = codeElement.scrollHeight;
    }
}

function setupButtonCloseState(button, projectId, modalType) {
    button.textContent = 'Close';
    button.disabled = false;
    button.onclick = function () {
        // Close the modal instead of reloading the page
        const modalId = `${modalType}-project-modal-${projectId}`;
        closeModal(modalId);

        // Update the project card with the stored state information after modal closes
        setTimeout(() => {
            updateProjectCardWithState(projectId);
        }, 100); // Small delay to ensure modal close completes
    };
}

function addOarText(outputContent, text, type = '') {
    // Add separation after Docker Compose output if the last message was Docker Compose
    const needsSeparationBefore = outputContent.lastWasDockerCompose;
    const prefix = needsSeparationBefore ? '\n' : '';

    const className = type ? `oar-text ${type}` : 'oar-text';
    outputContent.innerHTML += `${prefix}<span class="${className}" style="font-style: italic;">${text}</span>\n`;
    outputContent.lastWasDockerCompose = false;
}

function addDockerComposeOutput(outputContent, text) {
    // Check if there's existing content and if the last message was not Docker Compose output
    const hasExistingContent = outputContent.innerHTML.trim().length > 0;
    const needsSeparationBefore = hasExistingContent && !outputContent.lastWasDockerCompose;

    // Add separation before Docker Compose output if needed
    const prefix = needsSeparationBefore ? '\n' : '';

    outputContent.innerHTML += prefix + text + '\n';
    outputContent.lastWasDockerCompose = true;
}

// Function to update a project card using stored state information
function updateProjectCardWithState(projectId) {
    // Get the stored project state
    const projectState = window.oarProjectStates && window.oarProjectStates[projectId];
    if (!projectState) {
        console.warn('No stored project state found for project:', projectId);
        return;
    }

    // Update the status badge
    updateProjectCardStatus(projectId, projectState.status);

    // Update last commit if provided
    if (projectState.lastCommit) {
        updateProjectCardCommit(projectId, projectState.lastCommit);
    }

    // Update action button visibility based on new status
    updateProjectActionButtons(projectId, projectState.status);

    // Clean up stored state
    delete window.oarProjectStates[projectId];
}

// Function to update project card status badge
function updateProjectCardStatus(projectId, newStatus) {
    const statusBadge = document.querySelector(`#project-card-${projectId} .status-badge`);
    if (!statusBadge) {
        console.warn('Status badge not found for project:', projectId);
        return;
    }

    // Remove all status-related classes
    statusBadge.classList.remove(
        'bg-green-200', 'text-green-800',    // running
        'bg-gray-200', 'text-gray-800',      // stopped
        'bg-red-200', 'text-red-800',        // error
        'bg-yellow-200', 'text-yellow-800'   // unknown
    );

    // Add appropriate classes based on new status
    switch (newStatus) {
        case 'running':
            statusBadge.classList.add('bg-green-200', 'text-green-800');
            break;
        case 'stopped':
            statusBadge.classList.add('bg-gray-200', 'text-gray-800');
            break;
        case 'error':
            statusBadge.classList.add('bg-red-200', 'text-red-800');
            break;
        default:
            statusBadge.classList.add('bg-yellow-200', 'text-yellow-800');
            newStatus = 'unknown';
    }

    // Update text content and data attribute
    statusBadge.textContent = newStatus;
    statusBadge.setAttribute('data-status', newStatus);
}

// Function to update project card commit badge
function updateProjectCardCommit(projectId, newCommit) {
    const commitBadge = document.querySelector(`#project-card-${projectId} .commit-badge`);
    if (!commitBadge) {
        console.warn('Commit badge not found for project:', projectId);
        return;
    }

    // Update classes for commit display
    commitBadge.classList.remove('bg-gray-100', 'text-gray-500'); // no commits style
    commitBadge.classList.add('bg-gray-200', 'text-gray-800');    // has commits style

    // Update text content (first 8 characters) and data attribute
    commitBadge.textContent = newCommit.substring(0, 8);
    commitBadge.setAttribute('data-commit', newCommit);
}

// Function to update project action buttons based on status
function updateProjectActionButtons(projectId, newStatus) {
    // Find the stop button container (now always rendered)
    const stopButtonTrigger = document.querySelector(`[data-modal-trigger="stop-project-modal-${projectId}"]`);

    if (stopButtonTrigger) {
        // Find the parent container (div.relative.group)
        const stopButtonContainer = stopButtonTrigger.closest('.relative.group');

        if (stopButtonContainer) {
            if (newStatus === 'running') {
                // Show stop button by removing hidden class
                stopButtonContainer.classList.remove('hidden');
            } else {
                // Hide stop button by adding hidden class
                stopButtonContainer.classList.add('hidden');
            }
        }
    }
}

// Generic function to close modal (moved from bottom of file)
function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        const closeButton = modal.querySelector('[data-modal-close]');
        if (closeButton) {
            closeButton.click();
        } else {
            // Fallback: manually trigger close
            modal.setAttribute('data-open', 'false');
            modal.style.display = 'none';
            modal.classList.add('hidden');
        }
    }
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

        try {
            // Parse JSON message
            const message = JSON.parse(data);

            // Handle project state updates
            if (message.projectState) {
                // Store project state for later use when modal closes
                if (!window.oarProjectStates) {
                    window.oarProjectStates = {};
                }
                window.oarProjectStates[projectId] = message.projectState;
            }

            // Handle display messages
            if (message.message) {
                if (message.type === 'info' && message.source === 'docker') {
                    // Docker Compose output - display as plain text with separation
                    addDockerComposeOutput(outputContent, message.message);
                } else {
                    // Oar messages (info/success/error) - display with italic styling
                    addOarText(outputContent, message.message, message.type);
                }
            }

            // Handle completion based on message type
            // Only treat success/error messages with projectState as completion
            if ((message.type === 'success' || message.type === 'error') && message.projectState) {
                // This is a completion message, close the stream and setup button
                eventSource.close();

                if (button) {
                    setupButtonCloseState(button, projectId, streamType);
                }

                if (onComplete) {
                    onComplete(message.type, message.message);
                }
            }
        } catch (e) {
            // Fallback for non-JSON messages (shouldn't happen with new unified format)
            console.warn('Received non-JSON message:', data);
            outputContent.innerHTML += data + '\n';
        }

        autoScroll(outputContent);
    };

    // Handle connection errors
    eventSource.onerror = function (event) {
        const errorMessage =
            streamType === 'logs'
                ? 'Connection error or log stream ended'
                : 'Connection error or process completed';

        addOarText(outputContent, errorMessage, 'error');
        eventSource.close();

        if (button) {
            setupButtonCloseState(button, projectId, streamType);
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
