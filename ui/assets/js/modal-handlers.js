/**
 * Modal Management for Project Operations
 * Handles closing modals after successful project operations based on HX-Trigger headers
 */

function closeModalBySelector(selector) {
    const modal = document.querySelector(selector);
    if (modal) {
        const closeButton = modal.querySelector('[data-modal-close]');
        if (closeButton) {
            closeButton.click();
        } else {
            // Fallback: manually hide the modal
            modal.style.display = 'none';
            modal.classList.add('hidden');
            modal.setAttribute('data-open', 'false');
        }
    }
}

function closeModalsByPrefix(prefix) {
    const modals = document.querySelectorAll(`[id^="${prefix}"]`);
    modals.forEach(modal => {
        const closeButton = modal.querySelector('[data-modal-close]');
        if (closeButton) {
            closeButton.click();
        }
    });
}

function handleProjectModalClosing(xhr) {
    if (xhr.status !== 200) return;
    
    const hxTrigger = xhr.getResponseHeader('HX-Trigger');
    if (!hxTrigger) return;

    if (hxTrigger.startsWith('project-updated-')) {
        const projectId = hxTrigger.replace('project-updated-', '');
        closeModalBySelector(`#edit-project-modal-${projectId}`);
    } else if (hxTrigger === 'project-created') {
        closeModalBySelector('#add-project-modal');
    } else if (hxTrigger === 'project-deleted') {
        closeModalsByPrefix('delete-project-modal-');
    } else if (hxTrigger === 'project-started') {
        closeModalsByPrefix('start-project-modal-');
    } else if (hxTrigger === 'project-stopped') {
        closeModalsByPrefix('stop-project-modal-');
    }
}

// Initialize modal handlers
document.addEventListener('DOMContentLoaded', function() {
    document.body.addEventListener('htmx:afterRequest', function(e) {
        handleProjectModalClosing(e.detail.xhr);
    });
});