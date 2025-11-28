/*
 * Copyright (c) 2015 Sylvain Peyrefitte
 *
 * This file is part of mstsc.js.
 *
 * mstsc.js is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

(function() {
	
	/**
	 * decompress bitmap from RLE algorithm
	 * @param	bitmap	{object} bitmap object of bitmap event of node-rdpjs
	 */
	function decompress (bitmap) {
		var fName = null;
		switch (bitmap.bitsPerPixel) {
		case 15:
			fName = 'bitmap_decompress_15';
			break;
		case 16:
			fName = 'bitmap_decompress_16';
			break;
		case 24:
			fName = 'bitmap_decompress_24';
			break;
		case 32:
			fName = 'bitmap_decompress_32';
			break;
		default:
			throw 'invalid bitmap data format';
		}
		
		var input = new Uint8Array(bitmap.data);
		var inputPtr = Module._malloc(input.length);
		var inputHeap = new Uint8Array(Module.HEAPU8.buffer, inputPtr, input.length);
		inputHeap.set(input);
		
		var output_width = bitmap.destRight - bitmap.destLeft + 1;
		var output_height = bitmap.destBottom - bitmap.destTop + 1;
		var ouputSize = output_width * output_height * 4;
		var outputPtr = Module._malloc(ouputSize);

		var outputHeap = new Uint8Array(Module.HEAPU8.buffer, outputPtr, ouputSize);

		var res = Module.ccall(fName,
			'number',
			['number', 'number', 'number', 'number', 'number', 'number', 'number', 'number'],
			[outputHeap.byteOffset, output_width, output_height, bitmap.width, bitmap.height, inputHeap.byteOffset, input.length]
		);
		
		var output = new Uint8ClampedArray(outputHeap.buffer, outputHeap.byteOffset, ouputSize);
		
		Module._free(inputPtr);
		Module._free(outputPtr);
		
		return { width : output_width, height : output_height, data : output };
	}
	
	/**
	 * Un compress bitmap are reverse in y axis
	 */
	function reverse (bitmap) {
		return { width : bitmap.width, height : bitmap.height, data : new Uint8ClampedArray(bitmap.data) };
	}

	/**
	 * Canvas renderer
	 * @param canvas {canvas} use for rendering
	 */
	function Canvas(canvas) {
		this.canvas = canvas;
		this.ctx = canvas.getContext("2d");
		// 1. 存储待处理更新的队列
        this.updateQueue = [];
        // 2. 标记是否已经请求了下一帧
        this.isRAFScheduled = false;
		this.render = this.render.bind(this);
		this.count=1;
	}
	
	Canvas.prototype = {
		/**
		 * update canvas with new bitmap
		 * @param bitmap {object}
		 */
		update : function (bitmap) {
			var output = null;
			if (bitmap.isCompress) {
				output = decompress(bitmap);
			}
			else {
				output = reverse(bitmap);
			}
			//console.log(bitmap);
			//console.log("-------------");
			//console.log(output);
			// use image data to use asm.js
			//var imageData = this.ctx.createImageData(output.width, output.height);
			var imageData = this.ctx.getImageData(bitmap.destLeft, bitmap.destTop,output.width, output.height);
			imageData.data.set(output.data);
			this.ctx.putImageData(imageData, bitmap.destLeft, bitmap.destTop);

			if(this.count>3000){
				/*
				const randomColor = getRandomColor();
				this.ctx.strokeStyle = randomColor; // 红色边框
				this.ctx.lineWidth = 2;       // 2像素粗
				this.ctx.setLineDash([5, 5]); // 虚线

				// 绘制目标矩形 (DestRect)
				// 目标坐标：(X, Y)
				// 目标尺寸：(Cx, Cy)
				this.ctx.strokeRect(bitmap.destLeft, bitmap.destTop,output.width, output.height); 
				*/
			}
			this.count++;

		},
		scrBltOrder:function(scrBltOrder){
			if (scrBltOrder.Cx <= 0 || scrBltOrder.Cy <= 0) {
				console.error('Invalid dimensions for ScrBltOrder');
				return;
			}
			// 使用drawImage复制屏幕区域
			/*
			this.ctx.drawImage(
				this.canvas,
				scrBltOrder.Srcx, scrBltOrder.Srcy, scrBltOrder.Cx, scrBltOrder.Cy,
				scrBltOrder.X, scrBltOrder.Y, scrBltOrder.Cx, scrBltOrder.Cy
			);*/
			


			// 2. [调试代码] 添加边框，可视化目标区域 (DestRect)
			/*
			this.ctx.strokeStyle = 'red'; // 红色边框
			this.ctx.lineWidth = 2;       // 2像素粗
			this.ctx.setLineDash([5, 5]); // 虚线

			// 绘制目标矩形 (DestRect)
			// 目标坐标：(X, Y)
			// 目标尺寸：(Cx, Cy)
			this.ctx.strokeRect(scrBltOrder.X, scrBltOrder.Y, scrBltOrder.Cx,  scrBltOrder.Cy); 
			*/
			
			// 绘制源矩形 (SrcRect) - 可选，用于对比
		//	this.ctx.strokeStyle = 'blue';
		//	this.ctx.setLineDash([2, 4]); 
		//	this.ctx.strokeRect(scrBltOrder.Srcx, scrBltOrder.Srcy, scrBltOrder.Cx, scrBltOrder.Cy);
			
			const imageData = this.ctx.getImageData(
				scrBltOrder.Srcx, scrBltOrder.Srcy, 
				scrBltOrder.Cx, scrBltOrder.Cy
			);
			this.ctx.putImageData(
				imageData, 
				scrBltOrder.X, scrBltOrder.Y
			);
		},
		render() {
			// 1. 标记 rAF 已执行，允许再次调度
			this.isRAFScheduled = false; 
			
			// 2. 取出并清空当前所有待处理的更新
			const updates = this.updateQueue;
			this.updateQueue = [];
			// 3. 批量执行所有更新
			for (const update of updates) {
				switch (update.type) {
					case 'scrblt':
						this.scrBltOrder(update.data);
						break;
					case 'bitmap':
						this.update(update.data);
						break;
				}
			}
    	},
		scheduleRender() {
			if (!this.isRAFScheduled) {
				// 确保只调用一次 requestAnimationFrame
				requestAnimationFrame(this.render);
				this.isRAFScheduled = true;
			}
    	},
		pushUpdate(type, bitmapData) {
			this.updateQueue.push({
				type: type,
				data: bitmapData
			});
			this.scheduleRender();
		}
	}
	
	/**
	 * Module export
	 */
	Mstsc.Canvas = {
		create : function (canvas) {
			return new Canvas(canvas);
		}
	}
})();
function getRandomColor() {
    // Math.random().toString(16) 将随机数转换为十六进制字符串
    // slice(2, 8) 截取小数点后的 6 位字符 (RRGGBB)
    // padStart(6, '0') 确保颜色代码是完整的 6 位
    return '#' + Math.floor(Math.random() * 16777215).toString(16).padStart(6, '0');
}