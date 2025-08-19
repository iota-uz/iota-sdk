/**
 * Permission Management System
 * Handles hierarchical permission toggles for roles management
 */

// Shared CSS classes for toggle switch visual states
const TOGGLE_CLASSES = {
    checked: "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-brand-600 after:translate-x-full after:border-white",
    unchecked: "relative w-11 h-6 rounded-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border after:rounded-full after:h-5 after:w-5 after:transition-all bg-gray-200 after:border-gray-300"
};

/**
 * Creates Alpine.js data for managing a permission set (group of related permissions)
 * Each set has a unique ID to prevent cross-contamination when toggling
 */
function createPermissionSetData(allChecked, someChecked, permissionIds, setId) {

    return {
        // Component state
        expanded: false,
        allChecked: allChecked,
        someChecked: someChecked,
        permissionIds: permissionIds,
        setId: setId,
        
        // Initialize component
        init() {
            this.$nextTick(() => this.updateState());
        },
        
        // Toggle all permissions in this set on/off
        toggleAll() {
            const newState = !this.allChecked;
            this.setAllPermissions(newState);
        },
        
        // Set all permissions to a specific state
        setAllPermissions(checked) {
            this.allChecked = checked;
            this.someChecked = false;
            
            // Update each permission checkbox
            const checkboxes = this.getPermissionCheckboxes();
            checkboxes.forEach(checkbox => {
                if (checkbox) {
                    checkbox.checked = checked;
                    checkbox.dispatchEvent(new Event('change', { bubbles: true }));
                }
            });
            
            this.updateVisualToggle();
        },
        
        // Get all checkbox elements for this permission set
        getPermissionCheckboxes() {
            return this.permissionIds.map(permId => 
                document.querySelector(`#${this.setId}-perm-${permId}`)
            );
        },
        
        // Update component state based on checkbox states
        updateState() {
            const checkboxes = this.getPermissionCheckboxes();
            const checkedCount = checkboxes.filter(cb => cb?.checked).length;
            const totalCount = checkboxes.filter(cb => cb !== null).length;
            
            this.allChecked = totalCount > 0 && checkedCount === totalCount;
            this.someChecked = checkedCount > 0 && checkedCount < totalCount;
            
            this.updateVisualToggle();
        },
        
        // Update the visual toggle switch appearance
        updateVisualToggle() {
            const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
            if (toggleVisual) {
                toggleVisual.className = this.allChecked 
                    ? TOGGLE_CLASSES.checked 
                    : TOGGLE_CLASSES.unchecked;
            }
        }
    };
}

/**
 * Creates Alpine.js data for managing an entire resource group (contains multiple permission sets)
 * Handles the resource-level "select all" functionality
 */
function createPermissionFormData(allChecked, someChecked, permissionIds) {

    return {
        // Component state
        allChecked: allChecked,
        someChecked: someChecked,
        permissionIds: permissionIds,
        
        // Initialize component
        init() {
            this.$nextTick(() => this.updateState());
        },
        
        // Toggle all permissions in this resource group on/off
        toggleAll() {
            const newState = !this.allChecked;
            this.setAllPermissions(newState);
            this.updateNestedPermissionSets(newState);
        },
        
        // Set all permissions to a specific state
        setAllPermissions(checked) {
            this.allChecked = checked;
            
            // Update all permission checkboxes in this resource
            const checkboxes = this.getPermissionCheckboxes();
            checkboxes.forEach(checkbox => {
                checkbox.checked = checked;
                checkbox.dispatchEvent(new Event('change', { bubbles: true }));
            });
            
            this.updateVisualToggle();
        },
        
        // Get all checkbox elements for this resource group
        getPermissionCheckboxes() {
            return this.permissionIds
                .map(permId => document.querySelector(`input[name="Permissions[${permId}]"]`))
                .filter(checkbox => checkbox !== null);
        },
        
        // Update nested permission set components to reflect new state
        updateNestedPermissionSets(checked) {
            const nestedComponents = this.$el.querySelectorAll('[x-data]');
            
            nestedComponents.forEach(el => {
                // Skip self and check if it's an Alpine component with our data structure
                if (el === this.$el) return;
                
                const alpineData = el._x_dataStack?.[0];
                if (alpineData && typeof alpineData.allChecked !== 'undefined') {
                    alpineData.allChecked = checked;
                    alpineData.someChecked = false;
                    alpineData.updateState?.();
                }
            });
        },
        
        // Update component state based on checkbox states
        updateState() {
            const checkboxes = this.getPermissionCheckboxes();
            const checkedCount = checkboxes.filter(cb => cb.checked).length;
            const totalCount = checkboxes.length;
            
            this.allChecked = totalCount > 0 && checkedCount === totalCount;
            this.someChecked = checkedCount > 0 && checkedCount < totalCount;
            
            this.updateVisualToggle();
        },
        
        // Update the visual toggle switch appearance
        updateVisualToggle() {
            const toggleVisual = this.$el.querySelector('[id^="toggle-visual-"]');
            if (toggleVisual) {
                toggleVisual.className = this.allChecked 
                    ? TOGGLE_CLASSES.checked 
                    : TOGGLE_CLASSES.unchecked;
            }
        }
    };
}