-- OMugen: controller indicator.
--
-- In menus, P1-P4 tags in the bottom-right corner show which players have a
-- controller (in their color, lit up while they press something), a notice
-- appears when a controller joins a player, and when several players have a
-- controller the highlighted menu item is tagged with the player driving the
-- menu. Players get controllers in mugen_input.go (MUGEN-style controls only).

local hud = {}

local W, H = 1280, 720
local colors = {{232, 52, 63}, {47, 123, 234}, {45, 190, 108}, {242, 182, 48}}
local font, text, back, fill
local toasts = {}

local function init()
	if font ~= nil then
		return
	end
	font = fontNew('data/omugen/fonts/b.def', 20)
	text = textImgNew()
	textImgSetFont(text, font)
	textImgSetBank(text, 0)
	textImgSetAlign(text, 0)
	textImgSetLayerno(text, 2)
	back, fill = rectNew(), rectNew()
	for _, r in ipairs({back, fill}) do
		rectSetLayerno(r, 2)
	end
end

-- box draws a filled rectangle in the given localcoord.
local function box(r, lw, lh, x1, y1, x2, y2, c, alpha)
	rectSetLocalcoord(r, lw, lh)
	rectSetColor(r, c[1], c[2], c[3])
	rectSetAlpha(r, alpha, 255 - alpha)
	rectSetWindow(r, x1, y1, x2, y2)
	rectUpdate(r)
	rectDraw(r)
end

local function label(lw, lh, s, x, y, scale, c, alpha)
	textImgSetLocalcoord(text, lw, lh)
	textImgSetScale(text, scale, scale)
	textImgSetColor(text, c[1], c[2], c[3], alpha)
	textImgSetText(text, s)
	textImgSetPos(text, x, y)
	textImgDraw(text)
end

-- tag draws a player tag ("P2") whose right edge is at x, centered on y.
local function tag(lw, lh, pn, x, y, scale, lit)
	local w, h = 46 * scale, 24 * scale
	local c = colors[pn]
	box(back, lw, lh, x - w, y - h / 2, x, y + h / 2, {0, 0, 0}, 150)
	box(fill, lw, lh, x - w + 2 * scale, y - h / 2 + 2 * scale, x - 2 * scale, y + h / 2 - 2 * scale, c, lit and 255 or 205)
	label(lw, lh, 'P' .. pn, x - w / 2, y + 7 * scale, scale, {255, 255, 255}, 255)
end

local function info()
	if omugenControllers == nil then
		return nil
	end
	local t = omugenControllers()
	if t == nil or (t.devices or 0) == 0 then
		return nil
	end
	return t
end

local function playersWithPads(t)
	local n = 0
	for pn = 1, 4 do
		if t[pn] ~= nil and t[pn].pad ~= nil and t[pn].pad ~= '' then
			n = n + 1
		end
	end
	return n
end

-- draw runs before every refresh() of a menu screen.
function hud.draw()
	local t = info()
	if t == nil then
		return
	end
	init()
	for _, j in ipairs(omugenTakeJoins()) do
		table.insert(toasts, {pn = j.player, t0 = os.clock()})
	end
	-- P1-P4 tags, bottom right; with several players, "selecting" marks the
	-- one driving the menus
	local x = W - 14
	local several = playersWithPads(t) >= 2
	for pn = 4, 1, -1 do
		local p = t[pn]
		if p ~= nil and p.wants then
			local hasPad = p.pad ~= nil and p.pad ~= ''
			if hasPad then
				tag(W, H, pn, x, H - 26, 1, p.recent)
				if several and t.active == pn then
					box(back, W, H, x - 56, H - 62, x + 10, H - 42, {0, 0, 0}, 190)
					box(fill, W, H, x - 56, H - 44, x + 10, H - 42, colors[pn], 255)
					label(W, H, 'SELECTING', x - 23, H - 48, 0.62, {255, 255, 255}, 255)
				end
			else
				box(back, W, H, x - 46, H - 38, x, H - 14, {0, 0, 0}, 120)
				label(W, H, 'P' .. pn, x - 23, H - 19, 1, {150, 150, 150}, 255)
			end
			x = x - 52
		end
	end
	-- "controller joined" notices, top center
	local now, y = os.clock(), 28
	for i = #toasts, 1, -1 do
		if now - toasts[i].t0 > 2.5 then
			table.remove(toasts, i)
		end
	end
	for _, n in ipairs(toasts) do
		local age = now - n.t0
		local alpha = math.floor(255 * math.min(1, (2.5 - age) / 0.4))
		local s = 'P' .. n.pn .. '  controller connected'
		local w = 300
		box(back, W, H, W / 2 - w / 2, y, W / 2 + w / 2, y + 34, {0, 0, 0}, math.floor(alpha * 0.7))
		box(fill, W, H, W / 2 - w / 2, y, W / 2 - w / 2 + 6, y + 34, colors[n.pn], alpha)
		label(W, H, s, W / 2 + 3, y + 24, 1, {255, 255, 255}, alpha)
		y = y + 42
	end
end

-- noteActiveItem remembers the text sprite of the highlighted menu item, as
-- just drawn (main.f_menuCommonDraw).
-- noteActiveItem records where the highlighted menu item was just drawn
-- (main.f_menuCommonDraw): its text, and its background picture if any.
local active
function hud.noteActiveItem(sprite, s, bg)
	active = nil
	if sprite == nil or s == nil or s == '' then
		return
	end
	local a = {}
	a.left, a.base, a.w, a.h = omugenTextBounds(sprite, s)
	if bg ~= nil then
		a.bx, a.by, a.bw, a.bh = omugenAnimBounds(bg)
	end
	active = a
end

-- menuBadge tags the highlighted menu item with the player driving the menu,
-- when several players have a controller, just left of the item's text
-- (wherever the screenpack puts it).
function hud.menuBadge()
	local a = active
	active = nil
	local t = info()
	if a == nil or t == nil or (t.active or 0) == 0 or playersWithPads(t) < 2 then
		return
	end
	init()
	-- Bounds are in the engine's 320x240 space (x from about -53 to 373 on a
	-- 16:9 screen), so the tag is drawn in it too.
	local lw, lh = 320, 240
	local scale = lh / H
	-- Menus drawn with picture fonts (each item a screen-sized glyph) don't
	-- say where the item is: the corner tags show who is selecting instead.
	if a.w > 200 or a.h > 120 or a.left + a.w < -60 or a.left > 380 or a.base < 0 or a.base - a.h > lh then
		return
	end
	local x, y = a.left - 8 * scale, a.base - a.h * 0.35
	-- Menus whose items are pictures (the words are part of the picture and
	-- the menu text is elsewhere or hidden): tag the picture's corner.
	if a.bx ~= nil and a.bw > 4 and a.bh > 4 and
		(a.left + a.w < a.bx or a.left > a.bx + a.bw or a.base < a.by or a.base - a.h > a.by + a.bh) then
		x, y = a.bx + 52 * scale, a.by + 18 * scale
	end
	tag(lw, lh, t.active, x, y, scale, false)
end

return hud
