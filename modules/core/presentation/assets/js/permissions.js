// Permission set management functionality
function createPermissionSetData(allChecked, someChecked, permissionIds) {
    return {
        expanded: false,
        allChecked: allChecked,
        someChecked: someChecked,
        permissionIds: permissionIds,
        init() {
            this.$nextTick(() => {
                this.updateState();
            });
        },
        toggleAll() {
            // Find all permission checkboxes for this set
            const checkboxes = [];
            this.permissionIds.forEach(permId => {
                const checkbox = document.querySelector('input[name="Permissions[' + permId + ']"]');
                if (checkbox) {
                    checkboxes.push(checkbox);
                }
            });
            
            // Determine new state
            const checkedCount = checkboxes.filter(cb => cb.checked).length;
            const newState = checkedCount < checkboxes.length;
            
            // Update all checkboxes
            checkboxes.forEach(cb => {
                cb.checked = newState;
                const event = new Event('change', { bubbles: true });
                cb.dispatchEvent(event);
            });
            
            this.updateState();
        },
        updateState() {
            // Find all permission checkboxes for this set
            const checkboxes = [];
            this.permissionIds.forEach(permId => {
                const checkbox = document.querySelector('input[name="Permissions[' + permId + ']"]');
                if (checkbox) {
                    checkboxes.push(checkbox);
                }
            });
            
            const checkedCount = checkboxes.filter(cb => cb.checked).length;
            this.allChecked = checkedCount === checkboxes.length && checkboxes.length > 0;
            this.someChecked = checkedCount > 0 && checkedCount < checkboxes.length;
            
            // Update toggle visual
            const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
            if (toggleVisual) {
                if (this.allChecked) {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-brand-600 after:translate-x-full after:border-white";
                } else {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-gray-200 after:border-gray-300";
                }
            }
        }
    };
}

// Form-level permission management functionality
function createPermissionFormData(allChecked, someChecked, permissionIds) {
    return {
        allChecked: allChecked,
        someChecked: someChecked,
        permissionIds: permissionIds,
        init() {
            // Set up mutation observer to watch for form field changes
            this.$nextTick(() => {
                this.updateState();
            });
        },
        toggleAll() {
            // First, determine what the new state should be
            // If some are checked but not all, we want to check all
            // If all are checked, we want to uncheck all
            // If none are checked, we want to check all
            const permissionCheckboxes = [];
            this.permissionIds.forEach(permId => {
                const checkbox = document.querySelector('input[name="Permissions[' + permId + ']"]');
                if (checkbox) {
                    permissionCheckboxes.push(checkbox);
                }
            });
            
            const checkedCount = permissionCheckboxes.filter(cb => cb.checked).length;
            const totalCount = permissionCheckboxes.length;
            
            // Determine new state
            let newState;
            if (checkedCount === totalCount && totalCount > 0) {
                // All are checked, uncheck all
                newState = false;
            } else {
                // Some or none are checked, check all
                newState = true;
            }
            
            this.allChecked = newState;
            
            // Update visual toggle
            const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
            if (toggleVisual) {
                if (this.allChecked) {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-brand-600 after:translate-x-full after:border-white";
                } else {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-gray-200 after:border-gray-300";
                }
            }
            
            // Update all found checkboxes
            permissionCheckboxes.forEach(cb => {
                cb.checked = newState;
                // Trigger change event
                const event = new Event('change', { bubbles: true });
                cb.dispatchEvent(event);
            });
            
            // Also update nested Alpine component states for permission sets
            const nestedComponents = this.$el.querySelectorAll('[x-data]');
            nestedComponents.forEach(el => {
                if (el !== this.$el && el._x_dataStack && el._x_dataStack[0]) {
                    if (typeof el._x_dataStack[0].allChecked !== 'undefined') {
                        el._x_dataStack[0].allChecked = this.allChecked;
                        el._x_dataStack[0].someChecked = false;
                        if (el._x_dataStack[0].updateState) {
                            el._x_dataStack[0].updateState();
                        }
                    }
                }
            });
        },
        updateState() {
            const permissionCheckboxes = [];
            this.permissionIds.forEach(permId => {
                const checkbox = document.querySelector('input[name="Permissions[' + permId + ']"]');
                if (checkbox) {
                    permissionCheckboxes.push(checkbox);
                }
            });
            
            const checkedCount = permissionCheckboxes.filter(cb => cb.checked).length;
            const totalCount = permissionCheckboxes.length;
            
            this.allChecked = checkedCount === totalCount && totalCount > 0;
            this.someChecked = checkedCount > 0 && checkedCount < totalCount;
            
            // Update visual toggle
            const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
            if (toggleVisual) {
                if (this.allChecked) {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-brand-600 after:translate-x-full after:border-white";
                } else {
                    toggleVisual.className = "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-gray-200 after:border-gray-300";
                }
            }
        }
    };
}