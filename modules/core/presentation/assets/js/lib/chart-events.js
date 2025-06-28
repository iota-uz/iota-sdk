/**
 * Chart Event Handling Library
 * Handles chart interactions and events for HTMX-based dashboards
 */

/**
 * Global chart event handler function
 * @param {string} panelId - The ID of the chart panel
 * @param {string} eventType - The type of event (dataPoint, legend, marker, etc.)
 * @param {Object} event - The event data from the chart library
 * @param {Object} chartContext - The chart context object
 * @param {Object} opts - Chart options and configuration
 * @param {Object} actionConfig - Action configuration for the event
 */
function handleChartEvent(panelId, eventType, event, chartContext, opts, actionConfig) {
    try {
        // Extract event context data
        const eventData = {
            panelId: panelId,
            eventType: eventType,
            chartType: opts?.config?.chart?.type || 'unknown',
            actionConfig: actionConfig
        };

        // Add specific data based on event type
        if (eventType === 'dataPoint' && event && event.dataPointIndex !== undefined) {
            eventData.dataPoint = {
                x: event.dataPointIndex,
                y: event.seriesIndex,
                seriesIndex: event.seriesIndex,
                dataIndex: event.dataPointIndex,
                label: chartContext?.w?.globals?.labels?.[event.dataPointIndex] || '',
                value: chartContext?.w?.globals?.series?.[event.seriesIndex]?.[event.dataPointIndex] || 0,
                color: chartContext?.w?.globals?.colors?.[event.seriesIndex] || ''
            };
            eventData.seriesIndex = event.seriesIndex;
            eventData.dataIndex = event.dataPointIndex;
            eventData.label = eventData.dataPoint.label;
            eventData.value = eventData.dataPoint.value;
            eventData.seriesName = chartContext?.w?.globals?.seriesNames?.[event.seriesIndex] || '';
        } else if (eventType === 'legend' && event && event.seriesIndex !== undefined) {
            eventData.seriesIndex = event.seriesIndex;
            eventData.seriesName = chartContext?.w?.globals?.seriesNames?.[event.seriesIndex] || '';
        } else if (eventType === 'marker' && event) {
            eventData.seriesIndex = event.seriesIndex;
            eventData.dataIndex = event.dataPointIndex;
            eventData.label = chartContext?.w?.globals?.labels?.[event.dataPointIndex] || '';
            eventData.value = chartContext?.w?.globals?.series?.[event.seriesIndex]?.[event.dataPointIndex] || 0;
            eventData.seriesName = chartContext?.w?.globals?.seriesNames?.[event.seriesIndex] || '';
        } else if (eventType === 'xAxisLabel' && event) {
            eventData.label = event.labelValue || '';
            eventData.categoryName = event.labelValue || '';
        } else if (eventType === 'click') {
            // For general chart clicks, we might not have specific data point info
            // Check if we have click coordinates that might tell us which data point was clicked
            if (event && event.dataPointIndex !== undefined) {
                // Even for general clicks, sometimes we get dataPointIndex
                const dataIndex = event.dataPointIndex;
                eventData.dataIndex = dataIndex;
                eventData.label = chartContext?.w?.globals?.labels?.[dataIndex] || '';
                eventData.value = chartContext?.w?.globals?.series?.[0]?.[dataIndex] || 0;
            } else if (chartContext?.w?.globals?.labels && chartContext.w.globals.labels.length > 0) {
                // Use first label as fallback for general click events
                eventData.label = chartContext.w.globals.labels[0] || '';
            }
        }

        // Always populate basic chart data for any event type
        if (!eventData.label && chartContext?.w?.globals?.labels?.length > 0) {
            eventData.label = chartContext.w.globals.labels[0] || '';
        }
        if (!eventData.seriesName && chartContext?.w?.globals?.seriesNames?.length > 0) {
            eventData.seriesName = chartContext.w.globals.seriesNames[0] || '';
        }

        // Send HTMX request to handle the event
        fetch(`/api/lens/events/chart/${panelId}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            },
            body: JSON.stringify(eventData)
        })
        .then(response => {
            // Handle HTMX trigger headers
            const hxTrigger = response.headers.get('HX-Trigger');
            const hxRedirect = response.headers.get('HX-Redirect');
            
            if (hxRedirect) {
                window.location.href = hxRedirect;
            } else if (hxTrigger) {
                try {
                    const triggerData = JSON.parse(hxTrigger);
                    document.dispatchEvent(new CustomEvent('htmx:trigger', {
                        detail: triggerData
                    }));
                } catch (e) {
                    console.error('Failed to parse HX-Trigger data:', e);
                }
            } else {
                // If no headers, try to get the data from the response body
                return response.json().then(data => {
                    if (data.headers && data.headers['HX-Trigger']) {
                        try {
                            const triggerData = JSON.parse(data.headers['HX-Trigger']);
                            document.dispatchEvent(new CustomEvent('htmx:trigger', {
                                detail: triggerData
                            }));
                        } catch (e) {
                            console.error('Failed to parse HX-Trigger from body:', e);
                        }
                    }
                });
            }
            
            return response.json();
        })
        .catch(error => {
            console.error('Chart event request failed:', error);
        });

    } catch (error) {
        console.error('Chart event handling error:', error);
    }
}

/**
 * Initialize chart event handling
 * Sets up global event handlers and HTMX event listeners
 */
function initializeChartEvents() {
    // Set global handler if not already defined
    if (!window.handleChartEvent) {
        window.handleChartEvent = handleChartEvent;
    }

    // Handle custom HTMX events from server responses
    document.addEventListener('htmx:trigger', function(event) {
        const detail = event.detail;
        
        if (detail.openWindow) {
            openWindow(detail.openWindow);
        } else if (detail.showModal) {
            showModal(detail.showModal);
        } else if (detail.updateDashboard) {
            updateDashboard(detail.updateDashboard);
        } else if (detail.customFunction) {
            executeCustomFunction(detail.customFunction);
        }
    });
}

/**
 * Opens a new window with the specified URL
 * @param {Object} windowData - Window configuration
 */
function openWindow(windowData) {
    window.open(windowData.url, '_blank');
}

/**
 * Shows a modal dialog
 * @param {Object} modalData - Modal configuration
 */
function showModal(modalData) {
    // Create modal overlay
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    
    // Create modal content
    const modal = document.createElement('div');
    modal.className = 'modal-content bg-white rounded-lg p-6 max-w-md mx-4 relative';
    
    // Close button
    const closeButton = document.createElement('button');
    closeButton.innerHTML = '×';
    closeButton.className = 'absolute top-2 right-2 text-gray-500 hover:text-gray-700 text-xl w-6 h-6 flex items-center justify-center';
    closeButton.onclick = () => document.body.removeChild(overlay);
    
    // Title
    const title = document.createElement('h3');
    title.className = 'text-lg font-semibold mb-4';
    title.textContent = modalData.title;
    
    // Content
    const content = document.createElement('div');
    content.className = 'modal-body';
    
    if (modalData.url) {
        // Load content via URL
        htmx.ajax('GET', modalData.url, {
            target: content,
            swap: 'innerHTML'
        });
    } else {
        content.innerHTML = modalData.content || '';
    }
    
    modal.appendChild(closeButton);
    modal.appendChild(title);
    modal.appendChild(content);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);
    
    // Close on overlay click
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) {
            document.body.removeChild(overlay);
        }
    });
}

/**
 * Updates dashboard variables and filters
 * @param {Object} updateData - Update configuration
 */
function updateDashboard(updateData) {
    // Update dashboard variables and filters
    if (updateData.variables) {
        // Apply variables to dashboard
        Object.keys(updateData.variables).forEach(key => {
            const inputs = document.querySelectorAll(`[name="${key}"]`);
            inputs.forEach(input => {
                input.value = updateData.variables[key];
            });
        });
    }
    
    if (updateData.filters) {
        // Apply filters to dashboard
        Object.keys(updateData.filters).forEach(key => {
            const filterInputs = document.querySelectorAll(`[data-filter="${key}"]`);
            filterInputs.forEach(input => {
                input.value = updateData.filters[key];
            });
        });
    }
    
    // Trigger dashboard refresh
    htmx.trigger(document.body, 'dashboard:refresh');
}

/**
 * Executes a custom JavaScript function
 * @param {Object} customData - Custom function configuration
 */
function executeCustomFunction(customData) {
    try {
        // Execute custom JavaScript function
        if (customData.function && typeof window[customData.function] === 'function') {
            window[customData.function](customData.variables, customData.context);
        } else {
            // Try to evaluate as JavaScript code
            const func = new Function('variables', 'context', customData.function);
            func(customData.variables, customData.context);
        }
    } catch (error) {
        console.error('Custom function execution error:', error);
    }
}

// Initialize chart events when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeChartEvents);
} else {
    initializeChartEvents();
}