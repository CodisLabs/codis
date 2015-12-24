function loadJQueryAdapter($) {
	/**
	 * The default HighchartsAdapter for jQuery
	 */
	return {

		/**
		 * Initialize the adapter by applying some extensions to jQuery
		 */
		init: function (pathAnim) {

			// extend the animate function to allow SVG animations
			var Fx = $.fx;

			$.extend($.easing, {
				easeOutQuad: function (x, t, b, c, d) {
					return -c * (t /= d) * (t - 2) + b;
				}
			});

			// extend some methods to check for elem.attr, which means it is a Highcharts SVG object
			$.each(['cur', '_default', 'width', 'height', 'opacity'], function (i, fn) {
				var obj = Fx.step,
					base;

				// Handle different parent objects
				if (fn === 'cur') {
					obj = Fx.prototype; // 'cur', the getter, relates to Fx.prototype

				} else if (fn === '_default' && $.Tween) { // jQuery 1.8 model
					obj = $.Tween.propHooks[fn];
					fn = 'set';
				}

				// Overwrite the method
				base = obj[fn];
				if (base) { // step.width and step.height don't exist in jQuery < 1.7

					// create the extended function replacement
					obj[fn] = function (effects) {

						var elem, fx;

						// Fx.prototype.cur does not use fx argument
						fx = i ? effects : this;

						// Don't run animations on textual properties like align (#1821)
						if (fx.prop === 'align') {
							return;
						}

						// shortcut
						elem = fx.elem;

						// Fx.prototype.cur returns the current value. The other ones are setters
						// and returning a value has no effect.
						if (elem.attr) { // is SVG element wrapper
							return elem.attr(
								fx.prop.replace('strokeWidth', 'stroke-width'), // #4721
								fn === 'cur' ? undefined : fx.now
							);
						}

						return base.apply(this, arguments); // use jQuery's built-in method
					};
				}
			});

			// Extend the opacity getter, needed for fading opacity with IE9 and jQuery 1.10+
			wrap($.cssHooks.opacity, 'get', function (proceed, elem, computed) {
				return elem.attr ? (elem.opacity || 0) : proceed.call(this, elem, computed);
			});

			// Define the setter function for d (path definitions)
			this.addAnimSetter('d', function (fx) {
				var elem = fx.elem,
					ends;

				// Normally start and end should be set in state == 0, but sometimes,
				// for reasons unknown, this doesn't happen. Perhaps state == 0 is skipped
				// in these cases
				if (!fx.started) {
					ends = pathAnim.init(elem, elem.d, elem.toD);
					fx.start = ends[0];
					fx.end = ends[1];
					fx.started = true;
				}

				// Interpolate each value of the path
				elem.attr('d', pathAnim.step(fx.start, fx.end, fx.pos, elem.toD));
			});

			/**
			 * Utility for iterating over an array. Parameters are reversed compared to jQuery.
			 * @param {Array} arr
			 * @param {Function} fn
			 */
			this.each = Array.prototype.forEach ?
					function each(arr, fn) { // modern browsers
						return Array.prototype.forEach.call(arr, fn);

					} :
					function each(arr, fn) { // legacy
						var i,
							len = arr.length;
						for (i = 0; i < len; i++) {
							if (fn.call(arr[i], arr[i], i, arr) === false) {
								return i;
							}
						}
					};

			/**
			 * Register Highcharts as a plugin in the respective framework
			 */
			$.fn.highcharts = function () {
				var constr = 'Chart', // default constructor
					args = arguments,
					options,
					ret;

				if (this[0]) {

					if (isString(args[0])) {
						constr = args[0];
						args = Array.prototype.slice.call(args, 1);
					}
					options = args[0];

					// Create the chart
					if (options !== UNDEFINED) {
						options.chart = options.chart || {};
						options.chart.renderTo = this[0];
						ret = new Highcharts[constr](options, args[1]);
						ret = this;
					}

					// When called without parameters or with the return argument, get a predefined chart
					if (options === UNDEFINED) {
						ret = charts[attr(this[0], 'data-highcharts-chart')];
					}
				}

				return ret;
			};

		},

		/**
		 * Add an animation setter for a specific property
		 */
		addAnimSetter: function (prop, setter) {
			// jQuery 1.8 style
			if ($.Tween) {
				$.Tween.propHooks[prop] = {
					set: setter
				};
			// pre 1.8
			} else {
				$.fx.step[prop] = setter;
			}
		},

		/**
		 * Downloads a script and executes a callback when done.
		 * @param {String} scriptLocation
		 * @param {Function} callback
		 */
		getScript: $.getScript,

		/**
		 * Return the index of an item in an array, or -1 if not found
		 */
		inArray: $.inArray,

		/**
		 * A direct link to jQuery methods. MooTools and Prototype adapters must be implemented for each case of method.
		 * @param {Object} elem The HTML element
		 * @param {String} method Which method to run on the wrapped element
		 */
		adapterRun: function (elem, method) {
			return $(elem)[method]();
		},

		/**
		 * Filter an array
		 */
		grep: $.grep,

		/**
		 * Map an array
		 * @param {Array} arr
		 * @param {Function} fn
		 */
		map: function (arr, fn) {
			//return jQuery.map(arr, fn);
			var results = [],
				i,
				len = arr.length;
			for (i = 0; i < len; i++) {
				results[i] = fn.call(arr[i], arr[i], i, arr);
			}
			return results;

		},

		/**
		 * Get the position of an element relative to the top left of the page
		 */
		offset: function (el) {
			return $(el).offset();
		},

		/**
		 * Add an event listener
		 * @param {Object} el A HTML element or custom object
		 * @param {String} event The event type
		 * @param {Function} fn The event handler
		 */
		addEvent: function (el, event, fn) {
			$(el).bind(event, fn);
		},

		/**
		 * Remove event added with addEvent
		 * @param {Object} el The object
		 * @param {String} eventType The event type. Leave blank to remove all events.
		 * @param {Function} handler The function to remove
		 */
		removeEvent: function (el, eventType, handler) {
			// workaround for jQuery issue with unbinding custom events:
			// http://forum.jQuery.com/topic/javascript-error-when-unbinding-a-custom-event-using-jQuery-1-4-2
			var func = doc.removeEventListener ? 'removeEventListener' : 'detachEvent';
			if (doc[func] && el && !el[func]) {
				el[func] = function () {};
			}

			$(el).unbind(eventType, handler);
		},

		/**
		 * Fire an event on a custom object
		 * @param {Object} el
		 * @param {String} type
		 * @param {Object} eventArguments
		 * @param {Function} defaultFunction
		 */
		fireEvent: function (el, type, eventArguments, defaultFunction) {
			var event = $.Event(type),
				detachedType = 'detached' + type,
				defaultPrevented;

			// Remove warnings in Chrome when accessing returnValue (#2790), layerX and layerY. Although Highcharts
			// never uses these properties, Chrome includes them in the default click event and
			// raises the warning when they are copied over in the extend statement below.
			//
			// To avoid problems in IE (see #1010) where we cannot delete the properties and avoid
			// testing if they are there (warning in chrome) the only option is to test if running IE.
			if (!isMS && eventArguments) {
				delete eventArguments.layerX;
				delete eventArguments.layerY;
				delete eventArguments.returnValue;
			}

			extend(event, eventArguments);

			// Prevent jQuery from triggering the object method that is named the
			// same as the event. For example, if the event is 'select', jQuery
			// attempts calling el.select and it goes into a loop.
			if (el[type]) {
				el[detachedType] = el[type];
				el[type] = null;
			}

			// Wrap preventDefault and stopPropagation in try/catch blocks in
			// order to prevent JS errors when cancelling events on non-DOM
			// objects. #615.
			$.each(['preventDefault', 'stopPropagation'], function (i, fn) {
				var base = event[fn];
				event[fn] = function () {
					try {
						base.call(event);
					} catch (e) {
						if (fn === 'preventDefault') {
							defaultPrevented = true;
						}
					}
				};
			});

			// trigger it
			$(el).trigger(event);

			// attach the method
			if (el[detachedType]) {
				el[type] = el[detachedType];
				el[detachedType] = null;
			}

			if (defaultFunction && !event.isDefaultPrevented() && !defaultPrevented) {
				defaultFunction(event);
			}
		},

		/**
		 * Extension method needed for MooTools
		 */
		washMouseEvent: function (e) {
			var ret = e.originalEvent || e;

			// computed by jQuery, needed by IE8
			if (ret.pageX === UNDEFINED) { // #1236
				ret.pageX = e.pageX;
				ret.pageY = e.pageY;
			}

			return ret;
		},

		/**
		 * Animate a HTML element or SVG element wrapper
		 * @param {Object} el
		 * @param {Object} params
		 * @param {Object} options jQuery-like animation options: duration, easing, callback
		 */
		animate: function (el, params, options) {
			var $el = $(el);
			if (!el.style) {
				el.style = {}; // #1881
			}
			if (params.d) {
				el.toD = params.d; // keep the array form for paths, used in $.fx.step.d
				params.d = 1; // because in jQuery, animating to an array has a different meaning
			}

			$el.stop();
			if (params.opacity !== UNDEFINED && el.attr) {
				params.opacity += 'px'; // force jQuery to use same logic as width and height (#2161)
			}
			el.hasAnim = 1; // #3342
			$el.animate(params, options);

		},
		/**
		 * Stop running animation
		 */
		stop: function (el) {
			if (el.hasAnim) { // #3342, memory leak on calling $(el) from destroy
				$(el).stop();
			}
		}
	};
}
