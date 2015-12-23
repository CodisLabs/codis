if (win.PointerEvent || win.MSPointerEvent) {

	// The touches object keeps track of the points being touched at all times
	var touches = {},
		hasPointerEvent = !!win.PointerEvent,
		getWebkitTouches = function () {
			var key, fake = [];
			fake.item = function (i) {
				return this[i];
			};
			for (key in touches) {
				if (touches.hasOwnProperty(key)) {
					fake.push({
						pageX: touches[key].pageX,
						pageY: touches[key].pageY,
						target: touches[key].target
					});
				}
			}
			return fake;
		},
		translateMSPointer = function (e, method, wktype, func) {
			var p;
			e = e.originalEvent || e;
			if ((e.pointerType === 'touch' || e.pointerType === e.MSPOINTER_TYPE_TOUCH) && charts[hoverChartIndex]) {
				func(e);
				p = charts[hoverChartIndex].pointer;
				p[method]({
					type: wktype,
					target: e.currentTarget,
					preventDefault: noop,
					touches: getWebkitTouches()
				});
			}
		};

	/**
	 * Extend the Pointer prototype with methods for each event handler and more
	 */
	extend(Pointer.prototype, {
		onContainerPointerDown: function (e) {
			translateMSPointer(e, 'onContainerTouchStart', 'touchstart', function (e) {
				touches[e.pointerId] = { pageX: e.pageX, pageY: e.pageY, target: e.currentTarget };
			});
		},
		onContainerPointerMove: function (e) {
			translateMSPointer(e, 'onContainerTouchMove', 'touchmove', function (e) {
				touches[e.pointerId] = { pageX: e.pageX, pageY: e.pageY };
				if (!touches[e.pointerId].target) {
					touches[e.pointerId].target = e.currentTarget;
				}
			});
		},
		onDocumentPointerUp: function (e) {
			translateMSPointer(e, 'onDocumentTouchEnd', 'touchend', function (e) {
				delete touches[e.pointerId];
			});
		},

		/**
		 * Add or remove the MS Pointer specific events
		 */
		batchMSEvents: function (fn) {
			fn(this.chart.container, hasPointerEvent ? 'pointerdown' : 'MSPointerDown', this.onContainerPointerDown);
			fn(this.chart.container, hasPointerEvent ? 'pointermove' : 'MSPointerMove', this.onContainerPointerMove);
			fn(doc, hasPointerEvent ? 'pointerup' : 'MSPointerUp', this.onDocumentPointerUp);
		}
	});

	// Disable default IE actions for pinch and such on chart element
	wrap(Pointer.prototype, 'init', function (proceed, chart, options) {
		proceed.call(this, chart, options);
		if (this.hasZoom) { // #4014
			css(chart.container, {
				'-ms-touch-action': NONE,
				'touch-action': NONE
			});
		}
	});

	// Add IE specific touch events to chart
	wrap(Pointer.prototype, 'setDOMEvents', function (proceed) {
		proceed.apply(this);
		if (this.hasZoom || this.followTouchMove) {
			this.batchMSEvents(addEvent);
		}
	});
	// Destroy MS events also
	wrap(Pointer.prototype, 'destroy', function (proceed) {
		this.batchMSEvents(removeEvent);
		proceed.call(this);
	});
}
