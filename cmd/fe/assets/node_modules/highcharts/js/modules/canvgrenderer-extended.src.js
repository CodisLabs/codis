/**
 * @license @product.name@ JS v@product.version@ (@product.date@)
 * CanVGRenderer Extension module
 *
 * (c) 2011-2012 Torstein Honsi, Erik Olsson
 *
 * License: www.highcharts.com/license
 */

(function (Highcharts) {
	var UNDEFINED,
		DIV = 'div',
		ABSOLUTE = 'absolute',
		RELATIVE = 'relative',
		HIDDEN = 'hidden',
		VISIBLE = 'visible',
		PX = 'px',
		css = Highcharts.css,
		CanVGRenderer = Highcharts.CanVGRenderer,
		SVGRenderer = Highcharts.SVGRenderer,
		extend = Highcharts.extend,
		merge = Highcharts.merge,
		addEvent = Highcharts.addEvent,
		createElement = Highcharts.createElement,
		discardElement = Highcharts.discardElement;

	// Extend CanVG renderer on demand, inherit from SVGRenderer
	extend(CanVGRenderer.prototype, SVGRenderer.prototype);

	// Add additional functionality:
	extend(CanVGRenderer.prototype, {
		create: function (chart, container, chartWidth, chartHeight) {
			this.setContainer(container, chartWidth, chartHeight);
			this.configure(chart);
		},
		setContainer: function (container, chartWidth, chartHeight) {
			var containerStyle = container.style,
				containerParent = container.parentNode,
				containerLeft = containerStyle.left,
				containerTop = containerStyle.top,
				containerOffsetWidth = container.offsetWidth,
				containerOffsetHeight = container.offsetHeight,
				canvas,
				initialHiddenStyle = { visibility: HIDDEN, position: ABSOLUTE };

			this.init(container, chartWidth, chartHeight);

			// add the canvas above it
			canvas = createElement('canvas', {
				width: containerOffsetWidth,
				height: containerOffsetHeight
			}, {
				position: RELATIVE,
				left: containerLeft,
				top: containerTop
			}, container);
			this.canvas = canvas;

			// Create the tooltip line and div, they are placed as siblings to
			// the container (and as direct childs to the div specified in the html page)
			this.ttLine = createElement(DIV, null, initialHiddenStyle, containerParent);
			this.ttDiv = createElement(DIV, null, initialHiddenStyle, containerParent);
			this.ttTimer = UNDEFINED;

			// Move away the svg node to a new div inside the container's parent so we can hide it.
			var hiddenSvg = createElement(DIV, {
				width: containerOffsetWidth,
				height: containerOffsetHeight
			}, {
				visibility: HIDDEN,
				left: containerLeft,
				top: containerTop
			}, containerParent);
			this.hiddenSvg = hiddenSvg;
			hiddenSvg.appendChild(this.box);
		},

		/**
		 * Configures the renderer with the chart. Attach a listener to the event tooltipRefresh.
		 **/
		configure: function (chart) {
			var renderer = this,
				options = chart.options.tooltip,
				borderWidth = options.borderWidth,
				tooltipDiv = renderer.ttDiv,
				tooltipDivStyle = options.style,
				tooltipLine = renderer.ttLine,
				padding = parseInt(tooltipDivStyle.padding, 10);

			// Add border styling from options to the style
			tooltipDivStyle = merge(tooltipDivStyle, {
				padding: padding + PX,
				'background-color': options.backgroundColor,
				'border-style': 'solid',
				'border-width': borderWidth + PX,
				'border-radius': options.borderRadius + PX
			});

			// Optionally add shadow
			if (options.shadow) {
				tooltipDivStyle = merge(tooltipDivStyle, {
					'box-shadow': '1px 1px 3px gray', // w3c
					'-webkit-box-shadow': '1px 1px 3px gray' // webkit
				});
			}
			css(tooltipDiv, tooltipDivStyle);

			// Set simple style on the line
			css(tooltipLine, {
				'border-left': '1px solid darkgray'
			});

			// This event is triggered when a new tooltip should be shown
			addEvent(chart, 'tooltipRefresh', function (args) {
				var chartContainer = chart.container,
					offsetLeft = chartContainer.offsetLeft,
					offsetTop = chartContainer.offsetTop,
					position;

				// Set the content of the tooltip
				tooltipDiv.innerHTML = args.text;

				// Compute the best position for the tooltip based on the divs size and container size.
				position = chart.tooltip.getPosition(
					tooltipDiv.offsetWidth, 
					tooltipDiv.offsetHeight, 
					{ plotX: args.x, plotY: args.y }
				);

				css(tooltipDiv, {
					visibility: VISIBLE,
					left: position.x + PX,
					top: position.y + PX,
					'border-color': args.borderColor
				});

				// Position the tooltip line
				css(tooltipLine, {
					visibility: VISIBLE,
					left: offsetLeft + args.x + PX,
					top: offsetTop + chart.plotTop + PX,
					height: chart.plotHeight  + PX
				});

				// This timeout hides the tooltip after 3 seconds
				// First clear any existing timer
				if (renderer.ttTimer !== UNDEFINED) {
					clearTimeout(renderer.ttTimer);
				}

				// Start a new timer that hides tooltip and line
				renderer.ttTimer = setTimeout(function () {
					css(tooltipDiv, { visibility: HIDDEN });
					css(tooltipLine, { visibility: HIDDEN });
				}, 3000);
			});
		},

		/**
		 * Extend SVGRenderer.destroy to also destroy the elements added by CanVGRenderer.
		 */
		destroy: function () {
			var renderer = this;

			// Remove the canvas
			discardElement(renderer.canvas);

			// Kill the timer
			if (renderer.ttTimer !== UNDEFINED) {
				clearTimeout(renderer.ttTimer);
			}

			// Remove the divs for tooltip and line
			discardElement(renderer.ttLine);
			discardElement(renderer.ttDiv);
			discardElement(renderer.hiddenSvg);

			// Continue with base class
			return SVGRenderer.prototype.destroy.apply(renderer);
		},

		/**
		 * Take a color and return it if it's a string, do not make it a gradient even if it is a
		 * gradient. Currently canvg cannot render gradients (turns out black),
		 * see: http://code.google.com/p/canvg/issues/detail?id=104
		 *
		 * @param {Object} color The color or config object
		 */
		color: function (color, elem, prop) {
			if (color && color.linearGradient) {
				// Pick the end color and forward to base implementation
				color = color.stops[color.stops.length - 1][1];
			}
			return SVGRenderer.prototype.color.call(this, color, elem, prop);
		},

		/**
		 * Draws the SVG on the canvas or adds a draw invokation to the deferred list.
		 */
		draw: function () {
			var renderer = this;
			window.canvg(renderer.canvas, renderer.hiddenSvg.innerHTML);
		}
	});
}(Highcharts));
