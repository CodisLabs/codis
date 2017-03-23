

/* ****************************************************************************
 *                                                                            *
 * START OF INTERNET EXPLORER <= 8 SPECIFIC CODE                              *
 *                                                                            *
 * For applications and websites that don't need IE support, like platform    *
 * targeted mobile apps and web apps, this code can be removed.               *
 *                                                                            *
 *****************************************************************************/

/**
 * @constructor
 */
var VMLRenderer, VMLElement;
if (!hasSVG && !useCanVG) {

/**
 * The VML element wrapper.
 */
VMLElement = {

	/**
	 * Initialize a new VML element wrapper. It builds the markup as a string
	 * to minimize DOM traffic.
	 * @param {Object} renderer
	 * @param {Object} nodeName
	 */
	init: function (renderer, nodeName) {
		var wrapper = this,
			markup =  ['<', nodeName, ' filled="f" stroked="f"'],
			style = ['position: ', ABSOLUTE, ';'],
			isDiv = nodeName === DIV;

		// divs and shapes need size
		if (nodeName === 'shape' || isDiv) {
			style.push('left:0;top:0;width:1px;height:1px;');
		}
		style.push('visibility: ', isDiv ? HIDDEN : VISIBLE);

		markup.push(' style="', style.join(''), '"/>');

		// create element with default attributes and style
		if (nodeName) {
			markup = isDiv || nodeName === 'span' || nodeName === 'img' ?
				markup.join('')	:
				renderer.prepVML(markup);
			wrapper.element = createElement(markup);
		}

		wrapper.renderer = renderer;
	},

	/**
	 * Add the node to the given parent
	 * @param {Object} parent
	 */
	add: function (parent) {
		var wrapper = this,
			renderer = wrapper.renderer,
			element = wrapper.element,
			box = renderer.box,
			inverted = parent && parent.inverted,

			// get the parent node
			parentNode = parent ?
				parent.element || parent :
				box;

		if (parent) {
			this.parentGroup = parent;
		}

		// if the parent group is inverted, apply inversion on all children
		if (inverted) { // only on groups
			renderer.invertChild(element, parentNode);
		}

		// append it
		parentNode.appendChild(element);

		// align text after adding to be able to read offset
		wrapper.added = true;
		if (wrapper.alignOnAdd && !wrapper.deferUpdateTransform) {
			wrapper.updateTransform();
		}

		// fire an event for internal hooks
		if (wrapper.onAdd) {
			wrapper.onAdd();
		}

		return wrapper;
	},

	/**
	 * VML always uses htmlUpdateTransform
	 */
	updateTransform: SVGElement.prototype.htmlUpdateTransform,

	/**
	 * Set the rotation of a span with oldIE's filter
	 */
	setSpanRotation: function () {
		// Adjust for alignment and rotation. Rotation of useHTML content is not yet implemented
		// but it can probably be implemented for Firefox 3.5+ on user request. FF3.5+
		// has support for CSS3 transform. The getBBox method also needs to be updated
		// to compensate for the rotation, like it currently does for SVG.
		// Test case: http://jsfiddle.net/highcharts/Ybt44/

		var rotation = this.rotation,
			costheta = mathCos(rotation * deg2rad),
			sintheta = mathSin(rotation * deg2rad);

		css(this.element, {
			filter: rotation ? ['progid:DXImageTransform.Microsoft.Matrix(M11=', costheta,
				', M12=', -sintheta, ', M21=', sintheta, ', M22=', costheta,
				', sizingMethod=\'auto expand\')'].join('') : NONE
		});
	},

	/**
	 * Get the positioning correction for the span after rotating.
	 */
	getSpanCorrection: function (width, baseline, alignCorrection, rotation, align) {

		var costheta = rotation ? mathCos(rotation * deg2rad) : 1,
			sintheta = rotation ? mathSin(rotation * deg2rad) : 0,
			height = pick(this.elemHeight, this.element.offsetHeight),
			quad,
			nonLeft = align && align !== 'left';

		// correct x and y
		this.xCorr = costheta < 0 && -width;
		this.yCorr = sintheta < 0 && -height;

		// correct for baseline and corners spilling out after rotation
		quad = costheta * sintheta < 0;
		this.xCorr += sintheta * baseline * (quad ? 1 - alignCorrection : alignCorrection);
		this.yCorr -= costheta * baseline * (rotation ? (quad ? alignCorrection : 1 - alignCorrection) : 1);
		// correct for the length/height of the text
		if (nonLeft) {
			this.xCorr -= width * alignCorrection * (costheta < 0 ? -1 : 1);
			if (rotation) {
				this.yCorr -= height * alignCorrection * (sintheta < 0 ? -1 : 1);
			}
			css(this.element, {
				textAlign: align
			});
		}
	},

	/**
	 * Converts a subset of an SVG path definition to its VML counterpart. Takes an array
	 * as the parameter and returns a string.
	 */
	pathToVML: function (value) {
		// convert paths
		var i = value.length,
			path = [];

		while (i--) {

			// Multiply by 10 to allow subpixel precision.
			// Substracting half a pixel seems to make the coordinates
			// align with SVG, but this hasn't been tested thoroughly
			if (isNumber(value[i])) {
				path[i] = mathRound(value[i] * 10) - 5;
			} else if (value[i] === 'Z') { // close the path
				path[i] = 'x';
			} else {
				path[i] = value[i];

				// When the start X and end X coordinates of an arc are too close,
				// they are rounded to the same value above. In this case, substract or
				// add 1 from the end X and Y positions. #186, #760, #1371, #1410.
				if (value.isArc && (value[i] === 'wa' || value[i] === 'at')) {
					// Start and end X
					if (path[i + 5] === path[i + 7]) {
						path[i + 7] += value[i + 7] > value[i + 5] ? 1 : -1;
					}
					// Start and end Y
					if (path[i + 6] === path[i + 8]) {
						path[i + 8] += value[i + 8] > value[i + 6] ? 1 : -1;
					}
				}
			}
		}


		// Loop up again to handle path shortcuts (#2132)
		/*while (i++ < path.length) {
			if (path[i] === 'H') { // horizontal line to
				path[i] = 'L';
				path.splice(i + 2, 0, path[i - 1]);
			} else if (path[i] === 'V') { // vertical line to
				path[i] = 'L';
				path.splice(i + 1, 0, path[i - 2]);
			}
		}*/
		return path.join(' ') || 'x';
	},

	/**
	 * Set the element's clipping to a predefined rectangle
	 *
	 * @param {String} id The id of the clip rectangle
	 */
	clip: function (clipRect) {
		var wrapper = this,
			clipMembers,
			cssRet;

		if (clipRect) {
			clipMembers = clipRect.members;
			erase(clipMembers, wrapper); // Ensure unique list of elements (#1258)
			clipMembers.push(wrapper);
			wrapper.destroyClip = function () {
				erase(clipMembers, wrapper);
			};
			cssRet = clipRect.getCSS(wrapper);

		} else {
			if (wrapper.destroyClip) {
				wrapper.destroyClip();
			}
			cssRet = { clip: docMode8 ? 'inherit' : 'rect(auto)' }; // #1214
		}

		return wrapper.css(cssRet);

	},

	/**
	 * Set styles for the element
	 * @param {Object} styles
	 */
	css: SVGElement.prototype.htmlCss,

	/**
	 * Removes a child either by removeChild or move to garbageBin.
	 * Issue 490; in VML removeChild results in Orphaned nodes according to sIEve, discardElement does not.
	 */
	safeRemoveChild: function (element) {
		// discardElement will detach the node from its parent before attaching it
		// to the garbage bin. Therefore it is important that the node is attached and have parent.
		if (element.parentNode) {
			discardElement(element);
		}
	},

	/**
	 * Extend element.destroy by removing it from the clip members array
	 */
	destroy: function () {
		if (this.destroyClip) {
			this.destroyClip();
		}

		return SVGElement.prototype.destroy.apply(this);
	},

	/**
	 * Add an event listener. VML override for normalizing event parameters.
	 * @param {String} eventType
	 * @param {Function} handler
	 */
	on: function (eventType, handler) {
		// simplest possible event model for internal use
		this.element['on' + eventType] = function () {
			var evt = win.event;
			evt.target = evt.srcElement;
			handler(evt);
		};
		return this;
	},

	/**
	 * In stacked columns, cut off the shadows so that they don't overlap
	 */
	cutOffPath: function (path, length) {

		var len;

		path = path.split(/[ ,]/);
		len = path.length;

		if (len === 9 || len === 11) {
			path[len - 4] = path[len - 2] = pInt(path[len - 2]) - 10 * length;
		}
		return path.join(' ');
	},

	/**
	 * Apply a drop shadow by copying elements and giving them different strokes
	 * @param {Boolean|Object} shadowOptions
	 */
	shadow: function (shadowOptions, group, cutOff) {
		var shadows = [],
			i,
			element = this.element,
			renderer = this.renderer,
			shadow,
			elemStyle = element.style,
			markup,
			path = element.path,
			strokeWidth,
			modifiedPath,
			shadowWidth,
			shadowElementOpacity;

		// some times empty paths are not strings
		if (path && typeof path.value !== 'string') {
			path = 'x';
		}
		modifiedPath = path;

		if (shadowOptions) {
			shadowWidth = pick(shadowOptions.width, 3);
			shadowElementOpacity = (shadowOptions.opacity || 0.15) / shadowWidth;
			for (i = 1; i <= 3; i++) {

				strokeWidth = (shadowWidth * 2) + 1 - (2 * i);

				// Cut off shadows for stacked column items
				if (cutOff) {
					modifiedPath = this.cutOffPath(path.value, strokeWidth + 0.5);
				}

				markup = ['<shape isShadow="true" strokeweight="', strokeWidth,
					'" filled="false" path="', modifiedPath,
					'" coordsize="10 10" style="', element.style.cssText, '" />'];

				shadow = createElement(renderer.prepVML(markup),
					null, {
						left: pInt(elemStyle.left) + pick(shadowOptions.offsetX, 1),
						top: pInt(elemStyle.top) + pick(shadowOptions.offsetY, 1)
					}
				);
				if (cutOff) {
					shadow.cutOff = strokeWidth + 1;
				}

				// apply the opacity
				markup = ['<stroke color="', shadowOptions.color || 'black', '" opacity="', shadowElementOpacity * i, '"/>'];
				createElement(renderer.prepVML(markup), null, null, shadow);


				// insert it
				if (group) {
					group.element.appendChild(shadow);
				} else {
					element.parentNode.insertBefore(shadow, element);
				}

				// record it
				shadows.push(shadow);

			}

			this.shadows = shadows;
		}
		return this;
	},
	updateShadows: noop, // Used in SVG only

	setAttr: function (key, value) {
		if (docMode8) { // IE8 setAttribute bug
			this.element[key] = value;
		} else {
			this.element.setAttribute(key, value);
		}
	},
	classSetter: function (value) {
		// IE8 Standards mode has problems retrieving the className unless set like this
		this.element.className = value;
	},
	dashstyleSetter: function (value, key, element) {
		var strokeElem = element.getElementsByTagName('stroke')[0] ||
			createElement(this.renderer.prepVML(['<stroke/>']), null, null, element);
		strokeElem[key] = value || 'solid';
		this[key] = value; /* because changing stroke-width will change the dash length
			and cause an epileptic effect */
	},
	dSetter: function (value, key, element) {
		var i,
			shadows = this.shadows;
		value = value || [];
		this.d = value.join && value.join(' '); // used in getter for animation

		element.path = value = this.pathToVML(value);

		// update shadows
		if (shadows) {
			i = shadows.length;
			while (i--) {
				shadows[i].path = shadows[i].cutOff ? this.cutOffPath(value, shadows[i].cutOff) : value;
			}
		}
		this.setAttr(key, value);
	},
	fillSetter: function (value, key, element) {
		var nodeName = element.nodeName;
		if (nodeName === 'SPAN') { // text color
			element.style.color = value;
		} else if (nodeName !== 'IMG') { // #1336
			element.filled = value !== NONE;
			this.setAttr('fillcolor', this.renderer.color(value, element, key, this));
		}
	},
	opacitySetter: noop, // Don't bother - animation is too slow and filters introduce artifacts
	rotationSetter: function (value, key, element) {
		var style = element.style;
		this[key] = style[key] = value; // style is for #1873

		// Correction for the 1x1 size of the shape container. Used in gauge needles.
		style.left = -mathRound(mathSin(value * deg2rad) + 1) + PX;
		style.top = mathRound(mathCos(value * deg2rad)) + PX;
	},
	strokeSetter: function (value, key, element) {
		this.setAttr('strokecolor', this.renderer.color(value, element, key));
	},
	'stroke-widthSetter': function (value, key, element) {
		element.stroked = !!value; // VML "stroked" attribute
		this[key] = value; // used in getter, issue #113
		if (isNumber(value)) {
			value += PX;
		}
		this.setAttr('strokeweight', value);
	},
	titleSetter: function (value, key) {
		this.setAttr(key, value);
	},
	visibilitySetter: function (value, key, element) {

		// Handle inherited visibility
		if (value === 'inherit') {
			value = VISIBLE;
		}

		// Let the shadow follow the main element
		if (this.shadows) {
			each(this.shadows, function (shadow) {
				shadow.style[key] = value;
			});
		}

		// Instead of toggling the visibility CSS property, move the div out of the viewport.
		// This works around #61 and #586
		if (element.nodeName === 'DIV') {
			value = value === HIDDEN ? '-999em' : 0;

			// In order to redraw, IE7 needs the div to be visible when tucked away
			// outside the viewport. So the visibility is actually opposite of
			// the expected value. This applies to the tooltip only.
			if (!docMode8) {
				element.style[key] = value ? VISIBLE : HIDDEN;
			}
			key = 'top';
		}
		element.style[key] = value;
	},
	xSetter: function (value, key, element) {
		this[key] = value; // used in getter

		if (key === 'x') {
			key = 'left';
		} else if (key === 'y') {
			key = 'top';
		}/* else {
			value = mathMax(0, value); // don't set width or height below zero (#311)
		}*/

		// clipping rectangle special
		if (this.updateClipping) {
			this[key] = value; // the key is now 'left' or 'top' for 'x' and 'y'
			this.updateClipping();
		} else {
			// normal
			element.style[key] = value;
		}
	},
	zIndexSetter: function (value, key, element) {
		element.style[key] = value;
	}
};
Highcharts.VMLElement = VMLElement = extendClass(SVGElement, VMLElement);

// Some shared setters
VMLElement.prototype.ySetter =
	VMLElement.prototype.widthSetter =
	VMLElement.prototype.heightSetter =
	VMLElement.prototype.xSetter;


/**
 * The VML renderer
 */
var VMLRendererExtension = { // inherit SVGRenderer

	Element: VMLElement,
	isIE8: userAgent.indexOf('MSIE 8.0') > -1,


	/**
	 * Initialize the VMLRenderer
	 * @param {Object} container
	 * @param {Number} width
	 * @param {Number} height
	 */
	init: function (container, width, height, style) {
		var renderer = this,
			boxWrapper,
			box,
			css;

		renderer.alignedObjects = [];

		boxWrapper = renderer.createElement(DIV)
			.css(extend(this.getStyle(style), { position: 'relative' }));
		box = boxWrapper.element;
		container.appendChild(boxWrapper.element);


		// generate the containing box
		renderer.isVML = true;
		renderer.box = box;
		renderer.boxWrapper = boxWrapper;
		renderer.gradients = {};
		renderer.cache = {}; // Cache for numerical bounding boxes
		renderer.cacheKeys = [];


		renderer.setSize(width, height, false);

		// The only way to make IE6 and IE7 print is to use a global namespace. However,
		// with IE8 the only way to make the dynamic shapes visible in screen and print mode
		// seems to be to add the xmlns attribute and the behaviour style inline.
		if (!doc.namespaces.hcv) {

			doc.namespaces.add('hcv', 'urn:schemas-microsoft-com:vml');

			// Setup default CSS (#2153, #2368, #2384)
			css = 'hcv\\:fill, hcv\\:path, hcv\\:shape, hcv\\:stroke' +
				'{ behavior:url(#default#VML); display: inline-block; } ';
			try {
				doc.createStyleSheet().cssText = css;
			} catch (e) {
				doc.styleSheets[0].cssText += css;
			}

		}
	},


	/**
	 * Detect whether the renderer is hidden. This happens when one of the parent elements
	 * has display: none
	 */
	isHidden: function () {
		return !this.box.offsetWidth;
	},

	/**
	 * Define a clipping rectangle. In VML it is accomplished by storing the values
	 * for setting the CSS style to all associated members.
	 *
	 * @param {Number} x
	 * @param {Number} y
	 * @param {Number} width
	 * @param {Number} height
	 */
	clipRect: function (x, y, width, height) {

		// create a dummy element
		var clipRect = this.createElement(),
			isObj = isObject(x);

		// mimic a rectangle with its style object for automatic updating in attr
		return extend(clipRect, {
			members: [],
			count: 0,
			left: (isObj ? x.x : x) + 1,
			top: (isObj ? x.y : y) + 1,
			width: (isObj ? x.width : width) - 1,
			height: (isObj ? x.height : height) - 1,
			getCSS: function (wrapper) {
				var element = wrapper.element,
					nodeName = element.nodeName,
					isShape = nodeName === 'shape',
					inverted = wrapper.inverted,
					rect = this,
					top = rect.top - (isShape ? element.offsetTop : 0),
					left = rect.left,
					right = left + rect.width,
					bottom = top + rect.height,
					ret = {
						clip: 'rect(' +
							mathRound(inverted ? left : top) + 'px,' +
							mathRound(inverted ? bottom : right) + 'px,' +
							mathRound(inverted ? right : bottom) + 'px,' +
							mathRound(inverted ? top : left) + 'px)'
					};

				// issue 74 workaround
				if (!inverted && docMode8 && nodeName === 'DIV') {
					extend(ret, {
						width: right + PX,
						height: bottom + PX
					});
				}
				return ret;
			},

			// used in attr and animation to update the clipping of all members
			updateClipping: function () {
				each(clipRect.members, function (member) {
					if (member.element) { // Deleted series, like in stock/members/series-remove demo. Should be removed from members, but this will do.
						member.css(clipRect.getCSS(member));
					}
				});
			}
		});

	},


	/**
	 * Take a color and return it if it's a string, make it a gradient if it's a
	 * gradient configuration object, and apply opacity.
	 *
	 * @param {Object} color The color or config object
	 */
	color: function (color, elem, prop, wrapper) {
		var renderer = this,
			colorObject,
			regexRgba = /^rgba/,
			markup,
			fillType,
			ret = NONE;

		// Check for linear or radial gradient
		if (color && color.linearGradient) {
			fillType = 'gradient';
		} else if (color && color.radialGradient) {
			fillType = 'pattern';
		}


		if (fillType) {

			var stopColor,
				stopOpacity,
				gradient = color.linearGradient || color.radialGradient,
				x1,
				y1,
				x2,
				y2,
				opacity1,
				opacity2,
				color1,
				color2,
				fillAttr = '',
				stops = color.stops,
				firstStop,
				lastStop,
				colors = [],
				addFillNode = function () {
					// Add the fill subnode. When colors attribute is used, the meanings of opacity and o:opacity2
					// are reversed.
					markup = ['<fill colors="' + colors.join(',') + '" opacity="', opacity2, '" o:opacity2="', opacity1,
						'" type="', fillType, '" ', fillAttr, 'focus="100%" method="any" />'];
					createElement(renderer.prepVML(markup), null, null, elem);
				};

			// Extend from 0 to 1
			firstStop = stops[0];
			lastStop = stops[stops.length - 1];
			if (firstStop[0] > 0) {
				stops.unshift([
					0,
					firstStop[1]
				]);
			}
			if (lastStop[0] < 1) {
				stops.push([
					1,
					lastStop[1]
				]);
			}

			// Compute the stops
			each(stops, function (stop, i) {
				if (regexRgba.test(stop[1])) {
					colorObject = Color(stop[1]);
					stopColor = colorObject.get('rgb');
					stopOpacity = colorObject.get('a');
				} else {
					stopColor = stop[1];
					stopOpacity = 1;
				}

				// Build the color attribute
				colors.push((stop[0] * 100) + '% ' + stopColor);

				// Only start and end opacities are allowed, so we use the first and the last
				if (!i) {
					opacity1 = stopOpacity;
					color2 = stopColor;
				} else {
					opacity2 = stopOpacity;
					color1 = stopColor;
				}
			});

			// Apply the gradient to fills only.
			if (prop === 'fill') {

				// Handle linear gradient angle
				if (fillType === 'gradient') {
					x1 = gradient.x1 || gradient[0] || 0;
					y1 = gradient.y1 || gradient[1] || 0;
					x2 = gradient.x2 || gradient[2] || 0;
					y2 = gradient.y2 || gradient[3] || 0;
					fillAttr = 'angle="' + (90  - math.atan(
						(y2 - y1) / // y vector
						(x2 - x1) // x vector
						) * 180 / mathPI) + '"';

					addFillNode();

				// Radial (circular) gradient
				} else {

					var r = gradient.r,
						sizex = r * 2,
						sizey = r * 2,
						cx = gradient.cx,
						cy = gradient.cy,
						radialReference = elem.radialReference,
						bBox,
						applyRadialGradient = function () {
							if (radialReference) {
								bBox = wrapper.getBBox();
								cx += (radialReference[0] - bBox.x) / bBox.width - 0.5;
								cy += (radialReference[1] - bBox.y) / bBox.height - 0.5;
								sizex *= radialReference[2] / bBox.width;
								sizey *= radialReference[2] / bBox.height;
							}
							fillAttr = 'src="' + defaultOptions.global.VMLRadialGradientURL + '" ' +
								'size="' + sizex + ',' + sizey + '" ' +
								'origin="0.5,0.5" ' +
								'position="' + cx + ',' + cy + '" ' +
								'color2="' + color2 + '" ';

							addFillNode();
						};

					// Apply radial gradient
					if (wrapper.added) {
						applyRadialGradient();
					} else {
						// We need to know the bounding box to get the size and position right
						wrapper.onAdd = applyRadialGradient;
					}

					// The fill element's color attribute is broken in IE8 standards mode, so we
					// need to set the parent shape's fillcolor attribute instead.
					ret = color1;
				}

			// Gradients are not supported for VML stroke, return the first color. #722.
			} else {
				ret = stopColor;
			}

		// if the color is an rgba color, split it and add a fill node
		// to hold the opacity component
		} else if (regexRgba.test(color) && elem.tagName !== 'IMG') {

			colorObject = Color(color);

			markup = ['<', prop, ' opacity="', colorObject.get('a'), '"/>'];
			createElement(this.prepVML(markup), null, null, elem);

			ret = colorObject.get('rgb');


		} else {
			var propNodes = elem.getElementsByTagName(prop); // 'stroke' or 'fill' node
			if (propNodes.length) {
				propNodes[0].opacity = 1;
				propNodes[0].type = 'solid';
			}
			ret = color;
		}

		return ret;
	},

	/**
	 * Take a VML string and prepare it for either IE8 or IE6/IE7.
	 * @param {Array} markup A string array of the VML markup to prepare
	 */
	prepVML: function (markup) {
		var vmlStyle = 'display:inline-block;behavior:url(#default#VML);',
			isIE8 = this.isIE8;

		markup = markup.join('');

		if (isIE8) { // add xmlns and style inline
			markup = markup.replace('/>', ' xmlns="urn:schemas-microsoft-com:vml" />');
			if (markup.indexOf('style="') === -1) {
				markup = markup.replace('/>', ' style="' + vmlStyle + '" />');
			} else {
				markup = markup.replace('style="', 'style="' + vmlStyle);
			}

		} else { // add namespace
			markup = markup.replace('<', '<hcv:');
		}

		return markup;
	},

	/**
	 * Create rotated and aligned text
	 * @param {String} str
	 * @param {Number} x
	 * @param {Number} y
	 */
	text: SVGRenderer.prototype.html,

	/**
	 * Create and return a path element
	 * @param {Array} path
	 */
	path: function (path) {
		var attr = {
			// subpixel precision down to 0.1 (width and height = 1px)
			coordsize: '10 10'
		};
		if (isArray(path)) {
			attr.d = path;
		} else if (isObject(path)) { // attributes
			extend(attr, path);
		}
		// create the shape
		return this.createElement('shape').attr(attr);
	},

	/**
	 * Create and return a circle element. In VML circles are implemented as
	 * shapes, which is faster than v:oval
	 * @param {Number} x
	 * @param {Number} y
	 * @param {Number} r
	 */
	circle: function (x, y, r) {
		var circle = this.symbol('circle');
		if (isObject(x)) {
			r = x.r;
			y = x.y;
			x = x.x;
		}
		circle.isCircle = true; // Causes x and y to mean center (#1682)
		circle.r = r;
		return circle.attr({ x: x, y: y });
	},

	/**
	 * Create a group using an outer div and an inner v:group to allow rotating
	 * and flipping. A simple v:group would have problems with positioning
	 * child HTML elements and CSS clip.
	 *
	 * @param {String} name The name of the group
	 */
	g: function (name) {
		var wrapper,
			attribs;

		// set the class name
		if (name) {
			attribs = { 'className': PREFIX + name, 'class': PREFIX + name };
		}

		// the div to hold HTML and clipping
		wrapper = this.createElement(DIV).attr(attribs);

		return wrapper;
	},

	/**
	 * VML override to create a regular HTML image
	 * @param {String} src
	 * @param {Number} x
	 * @param {Number} y
	 * @param {Number} width
	 * @param {Number} height
	 */
	image: function (src, x, y, width, height) {
		var obj = this.createElement('img')
			.attr({ src: src });

		if (arguments.length > 1) {
			obj.attr({
				x: x,
				y: y,
				width: width,
				height: height
			});
		}
		return obj;
	},

	/**
	 * For rectangles, VML uses a shape for rect to overcome bugs and rotation problems
	 */
	createElement: function (nodeName) {
		return nodeName === 'rect' ? this.symbol(nodeName) : SVGRenderer.prototype.createElement.call(this, nodeName);
	},

	/**
	 * In the VML renderer, each child of an inverted div (group) is inverted
	 * @param {Object} element
	 * @param {Object} parentNode
	 */
	invertChild: function (element, parentNode) {
		var ren = this,
			parentStyle = parentNode.style,
			imgStyle = element.tagName === 'IMG' && element.style; // #1111

		css(element, {
			flip: 'x',
			left: pInt(parentStyle.width) - (imgStyle ? pInt(imgStyle.top) : 1),
			top: pInt(parentStyle.height) - (imgStyle ? pInt(imgStyle.left) : 1),
			rotation: -90
		});

		// Recursively invert child elements, needed for nested composite shapes like box plots and error bars. #1680, #1806.
		each(element.childNodes, function (child) {
			ren.invertChild(child, element);
		});
	},

	/**
	 * Symbol definitions that override the parent SVG renderer's symbols
	 *
	 */
	symbols: {
		// VML specific arc function
		arc: function (x, y, w, h, options) {
			var start = options.start,
				end = options.end,
				radius = options.r || w || h,
				innerRadius = options.innerR,
				cosStart = mathCos(start),
				sinStart = mathSin(start),
				cosEnd = mathCos(end),
				sinEnd = mathSin(end),
				ret;

			if (end - start === 0) { // no angle, don't show it.
				return ['x'];
			}

			ret = [
				'wa', // clockwise arc to
				x - radius, // left
				y - radius, // top
				x + radius, // right
				y + radius, // bottom
				x + radius * cosStart, // start x
				y + radius * sinStart, // start y
				x + radius * cosEnd, // end x
				y + radius * sinEnd  // end y
			];

			if (options.open && !innerRadius) {
				ret.push(
					'e',
					M,
					x,// - innerRadius,
					y// - innerRadius
				);
			}

			ret.push(
				'at', // anti clockwise arc to
				x - innerRadius, // left
				y - innerRadius, // top
				x + innerRadius, // right
				y + innerRadius, // bottom
				x + innerRadius * cosEnd, // start x
				y + innerRadius * sinEnd, // start y
				x + innerRadius * cosStart, // end x
				y + innerRadius * sinStart, // end y
				'x', // finish path
				'e' // close
			);

			ret.isArc = true;
			return ret;

		},
		// Add circle symbol path. This performs significantly faster than v:oval.
		circle: function (x, y, w, h, wrapper) {

			if (wrapper) {
				w = h = 2 * wrapper.r;
			}

			// Center correction, #1682
			if (wrapper && wrapper.isCircle) {
				x -= w / 2;
				y -= h / 2;
			}

			// Return the path
			return [
				'wa', // clockwisearcto
				x, // left
				y, // top
				x + w, // right
				y + h, // bottom
				x + w, // start x
				y + h / 2,     // start y
				x + w, // end x
				y + h / 2,     // end y
				//'x', // finish path
				'e' // close
			];
		},
		/**
		 * Add rectangle symbol path which eases rotation and omits arcsize problems
		 * compared to the built-in VML roundrect shape. When borders are not rounded,
		 * use the simpler square path, else use the callout path without the arrow.
		 */
		rect: function (x, y, w, h, options) {
			return SVGRenderer.prototype.symbols[
				!defined(options) || !options.r ? 'square' : 'callout'
			].call(0, x, y, w, h, options);
		}
	}
};
Highcharts.VMLRenderer = VMLRenderer = function () {
	this.init.apply(this, arguments);
};
VMLRenderer.prototype = merge(SVGRenderer.prototype, VMLRendererExtension);

	// general renderer
	Renderer = VMLRenderer;
}

// This method is used with exporting in old IE, when emulating SVG (see #2314)
SVGRenderer.prototype.measureSpanWidth = function (text, styles) {
	var measuringSpan = doc.createElement('span'),
		offsetWidth,
		textNode = doc.createTextNode(text);

	measuringSpan.appendChild(textNode);
	css(measuringSpan, styles);
	this.box.appendChild(measuringSpan);
	offsetWidth = measuringSpan.offsetWidth;
	discardElement(measuringSpan); // #2463
	return offsetWidth;
};


/* ****************************************************************************
 *                                                                            *
 * END OF INTERNET EXPLORER <= 8 SPECIFIC CODE                                *
 *                                                                            *
 *****************************************************************************/
