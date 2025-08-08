/**
 * Action Button Loading States and Git Auth Testing
 * Handles spinner states for action buttons and git authentication alerts
 */

function showButtonLoading(button, indicatorId) {
    const indicator = document.querySelector(indicatorId);
    const iconElement = button.querySelector('[id$="-icon"]');
    
    if (indicator && iconElement) {
        indicator.classList.remove('hidden');
        indicator.classList.add('block');
        iconElement.classList.remove('block');
        iconElement.classList.add('hidden');
    }
}

function hideButtonLoading(button, indicatorId) {
    const indicator = document.querySelector(indicatorId);
    const iconElement = button.querySelector('[id$="-icon"]');
    
    if (indicator && iconElement) {
        indicator.classList.remove('block');
        indicator.classList.add('hidden');
        iconElement.classList.remove('hidden');
        iconElement.classList.add('block');
    }
}

function handleTestAuthResponse(xhr, button) {
    if (button && button.getAttribute('hx-post') === '/test-git-auth') {
        const responseText = xhr.responseText;
        if (xhr.status === 200) {
            alert('🟢 ' + responseText);
        } else {
            alert('🔴 ' + responseText);
        }
    }
}

// Initialize action button handlers
document.addEventListener('DOMContentLoaded', function() {
    // Handle HTMX loading states for ActionButton spinners
    document.body.addEventListener('htmx:beforeRequest', function(e) {
        const button = e.detail.elt;
        const indicatorId = button.getAttribute('hx-indicator');
        if (indicatorId) {
            showButtonLoading(button, indicatorId);
        }
    });

    document.body.addEventListener('htmx:afterRequest', function(e) {
        const button = e.detail.elt;
        const indicatorId = button.getAttribute('hx-indicator');
        
        // Hide loading indicators
        if (indicatorId) {
            hideButtonLoading(button, indicatorId);
        }

        // Handle test auth response alerts
        handleTestAuthResponse(e.detail.xhr, button);
    });
});