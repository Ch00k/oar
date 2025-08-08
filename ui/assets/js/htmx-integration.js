/**
 * HTMX Integration and templUI Component Management
 * Handles re-initialization of templUI components after HTMX operations
 */

function reinitializeTemplUIComponents(element) {
    if (window.templUI) {
        Object.values(window.templUI).forEach(comp => {
            comp.init?.(element);
        });
    }
}

function cleanupTemplUIComponents(element) {
    if (window.templUI) {
        Object.values(window.templUI).forEach(comp => {
            comp.cleanup?.(element);
        });
    }
}

// Initialize HTMX integration handlers
document.addEventListener('DOMContentLoaded', function() {
    // Re-initialize templUI components after HTMX swaps
    document.body.addEventListener("htmx:afterSwap", (e) => {
        reinitializeTemplUIComponents(e.detail.elt);
    });

    // Re-initialize components after out-of-band swaps
    document.body.addEventListener("htmx:oobAfterSwap", (e) => {
        reinitializeTemplUIComponents(e.detail.target);
    });

    // Cleanup before swap (for components with event listeners)
    document.body.addEventListener("htmx:beforeSwap", (e) => {
        cleanupTemplUIComponents(e.detail.target);
    });
});