/**
 * Table of Contents Generator
 *
 * Dynamically generates a table of contents from page headings
 * and maintains active state based on scroll position.
 *
 * Features:
 * - Extracts H2-H4 headings from main content
 * - Builds nested list structure matching heading hierarchy
 * - Smooth scroll navigation
 * - IntersectionObserver for active section tracking
 * - Desktop-only (1280px+)
 * - Auto-hides if no headings found
 */

(function() {
  'use strict';

  // Only run on desktop screens
  if (window.innerWidth < 1280) {
    return;
  }

  // Get the main content and TOC container
  const mainContent = document.querySelector('#main-content main');
  const tocContent = document.querySelector('#toc-content');
  const tocSidebar = document.querySelector('#toc-sidebar');

  // Exit if elements not found
  if (!mainContent || !tocContent || !tocSidebar) {
    return;
  }

  // ========================================================================
  // Extract and Build TOC
  // ========================================================================

  /**
   * Get all headings from main content
   * @returns {Array} Array of heading elements
   */
  function getHeadings() {
    return Array.from(mainContent.querySelectorAll('h2, h3, h4'));
  }

  /**
   * Get heading level (2, 3, or 4)
   * @param {Element} heading - Heading element
   * @returns {number} Heading level
   */
  function getHeadingLevel(heading) {
    return parseInt(heading.tagName[1], 10);
  }

  /**
   * Get or create ID for heading
   * @param {Element} heading - Heading element
   * @returns {string} ID
   */
  function getHeadingId(heading) {
    if (heading.id) {
      return heading.id;
    }

    // Generate ID from heading text
    let id = heading.textContent
      .toLowerCase()
      .trim()
      .replace(/[^\w\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-');

    // Fallback for empty IDs
    if (!id) {
      id = `heading-${Math.random().toString(36).substring(2, 11)}`;
    }

    // Handle duplicate IDs by appending counter
    let finalId = id;
    let counter = 1;
    while (document.getElementById(finalId)) {
      finalId = `${id}-${counter}`;
      counter++;
    }

    heading.id = finalId;
    return heading.id;
  }

  /**
   * Build nested TOC HTML from headings
   * @param {Array} headings - Array of heading elements
   * @returns {string} HTML string
   */
  function buildTocHtml(headings) {
    if (headings.length === 0) {
      return '';
    }

    let html = '<ul>';
    let lastLevel = 2;

    headings.forEach((heading, index) => {
      const level = getHeadingLevel(heading);
      const id = getHeadingId(heading);
      const text = heading.textContent.trim();

      // Close previous level lists if going back up
      while (lastLevel > level) {
        html += '</li></ul>';
        lastLevel--;
      }

      // Open new level lists if going deeper
      if (level > lastLevel) {
        // Skip if jumping more than one level
        while (level > lastLevel + 1) {
          html += '<li><ul>';
          lastLevel++;
        }
        // Start new level
        if (lastLevel < level) {
          html += '<li><ul>';
          lastLevel++;
        }
      } else if (index > 0) {
        // Close previous item at same level
        html += '</li>';
      }

      // Add the current item
      html += `<li><a href="#${id}" class="toc-link toc-level-${level}" data-toc-id="${id}">${text}</a>`;
    });

    // Close all remaining open lists
    while (lastLevel >= 2) {
      html += '</li></ul>';
      lastLevel--;
    }

    return html;
  }

  /**
   * Initialize TOC
   */
  function initializeToc() {
    try {
      const headings = getHeadings();

      // Hide sidebar if no headings
      if (headings.length === 0) {
        tocSidebar.style.display = 'none';
        return;
      }

      // Build and insert TOC
      const tocHtml = buildTocHtml(headings);
      tocContent.innerHTML = tocHtml;

      // Setup click handlers for smooth scroll
      setupSmoothScroll();

      // Setup active state tracking
      setupActiveTracking(headings);
    } catch (error) {
      console.error('Error initializing TOC:', error);
      tocSidebar.style.display = 'none';
    }
  }

  // ========================================================================
  // Smooth Scroll Navigation
  // ========================================================================

  /**
   * Setup smooth scroll on TOC link clicks
   */
  function setupSmoothScroll() {
    const tocLinks = tocContent.querySelectorAll('.toc-link');

    tocLinks.forEach(link => {
      link.addEventListener('click', function(e) {
        e.preventDefault();

        const targetId = this.getAttribute('href').substring(1);
        const targetElement = document.getElementById(targetId);

        if (targetElement) {
          try {
            // Smooth scroll to target
            targetElement.scrollIntoView({
              behavior: 'smooth',
              block: 'start'
            });
          } catch (error) {
            // Fallback for browsers that don't support smooth scroll
            targetElement.scrollIntoView();
          }

          // Update active state immediately
          updateActiveState(targetId);
        }
      });
    });
  }

  // ========================================================================
  // Active State Tracking with IntersectionObserver
  // ========================================================================

  let activeHeadingId = null;

  /**
   * Update active state on a heading
   * @param {string} headingId - ID of the heading to mark as active
   */
  function updateActiveState(headingId) {
    // Remove active class from all links
    const allLinks = tocContent.querySelectorAll('.toc-link');
    allLinks.forEach(link => {
      link.classList.remove('toc-active');
    });

    // Add active class to current heading's link
    const activeLink = tocContent.querySelector(`[data-toc-id="${headingId}"]`);
    if (activeLink) {
      activeLink.classList.add('toc-active');
      activeHeadingId = headingId;
    }
  }

  // Store the IntersectionObserver instance for cleanup
  let headingObserver = null;

  /**
   * Setup IntersectionObserver for active section tracking
   * @param {Array} headings - Array of heading elements
   */
  function setupActiveTracking(headings) {
    // Clean up previous observer if exists
    if (headingObserver) {
      headingObserver.disconnect();
    }

    // Initial state: mark first heading as active
    if (headings.length > 0) {
      updateActiveState(getHeadingId(headings[0]));
    }

    // IntersectionObserver options
    // rootMargin: negative top margin for header offset, moderate bottom margin
    const observerOptions = {
      root: null,
      rootMargin: '-80px 0px -40% 0px',
      threshold: 0
    };

    /**
     * Intersection observer callback
     */
    const observerCallback = (entries) => {
      // Find the first intersecting heading
      let firstIntersecting = null;

      entries.forEach(entry => {
        if (entry.isIntersecting) {
          if (!firstIntersecting) {
            firstIntersecting = entry.target;
          } else {
            // Keep the topmost intersecting heading
            const firstRect = firstIntersecting.getBoundingClientRect();
            const entryRect = entry.target.getBoundingClientRect();

            if (entryRect.top < firstRect.top) {
              firstIntersecting = entry.target;
            }
          }
        }
      });

      // Update active state if a heading is visible
      if (firstIntersecting) {
        updateActiveState(getHeadingId(firstIntersecting));
      }
    };

    // Create observer
    headingObserver = new IntersectionObserver(observerCallback, observerOptions);

    // Observe all headings
    headings.forEach(heading => {
      headingObserver.observe(heading);
    });
  }

  // ========================================================================
  // Window Resize Handler
  // ========================================================================

  let resizeTimeout;
  let lastWidth = window.innerWidth;

  /**
   * Handle window resize - reinitialize if crossing desktop breakpoint
   */
  function handleWindowResize() {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
      const currentWidth = window.innerWidth;
      const wasDesktop = lastWidth >= 1280;
      const isDesktop = currentWidth >= 1280;

      // Only reinitialize if crossing the breakpoint
      if (wasDesktop !== isDesktop) {
        if (isDesktop) {
          // Switched to desktop, reinitialize
          initializeToc();
        } else {
          // Switched to mobile, cleanup and hide
          if (headingObserver) {
            headingObserver.disconnect();
          }
          if (contentObserver) {
            contentObserver.disconnect();
          }
          tocSidebar.style.display = 'none';
        }
        lastWidth = currentWidth;
      }
    }, 250);
  }

  window.addEventListener('resize', handleWindowResize);

  // ========================================================================
  // Initialize on page load
  // ========================================================================

  // Wait for DOM to be fully ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeToc);
  } else {
    initializeToc();
  }

  // ========================================================================
  // Dynamic Content Observer
  // ========================================================================

  let mutationTimeout;
  let contentObserver = null;

  /**
   * Handle dynamic content changes (e.g., if headings are added after page load)
   * Only observes on desktop screens
   */
  if (window.innerWidth >= 1280) {
    contentObserver = new MutationObserver(() => {
      clearTimeout(mutationTimeout);
      mutationTimeout = setTimeout(() => {
        if (window.innerWidth >= 1280) {
          initializeToc();
        }
      }, 500);
    });

    // Observe only heading additions/removals, not all content changes
    contentObserver.observe(mainContent, {
      childList: true,
      subtree: true,
      characterData: false,
      attributes: false
    });
  }
})();
