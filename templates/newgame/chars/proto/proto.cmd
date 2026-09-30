; Proto - commands
;
; [Command] sections name button sequences; [Statedef -1] at the end checks
; them every tick and starts the matching state. Directions are for a
; character facing right (F = toward the opponent): U D F B, DF DB UF UB.
; ~ means "released", $ "direction held (including diagonals)", / "held".
; Buttons: x y z (punches), a b c (kicks), s (start).

[Remap]
x = x
y = y
z = z
a = a
b = b
c = c
s = s

[Defaults]
command.time = 15
command.buffer.time = 1

;-| Special moves |-----------------------------------------------------------
[Command]
name = "overdrive"
command = ~D, DF, F, D, DF, F, x
time = 30

[Command]
name = "overdrive"
command = ~D, DF, F, D, DF, F, y
time = 30

[Command]
name = "blast"
command = ~D, DF, F, x

[Command]
name = "blast"
command = ~D, DF, F, y

;-| Required by the engine's common states (do not remove) |-------------------
[Command]
name = "FF"
command = F, F
time = 10

[Command]
name = "BB"
command = B, B
time = 10

[Command]
name = "recovery"
command = x+y
time = 1

[Command]
name = "holdfwd"
command = /$F
time = 1

[Command]
name = "holdback"
command = /$B
time = 1

[Command]
name = "holdup"
command = /$U
time = 1

[Command]
name = "holddown"
command = /$D
time = 1

;-| Single buttons |-----------------------------------------------------------
[Command]
name = "x"
command = x
time = 1

[Command]
name = "y"
command = y
time = 1

[Command]
name = "z"
command = z
time = 1

[Command]
name = "a"
command = a
time = 1

[Command]
name = "b"
command = b
time = 1

[Command]
name = "c"
command = c
time = 1

[Command]
name = "start"
command = s
time = 1

;===========================================================================
; Checked every tick, top to bottom: the first ChangeState that triggers
; wins, so stronger moves come first. "ctrl" means the player can act;
; "movecontact" lets a normal attack cancel into a special or super.
;===========================================================================
[Statedef -1]

[State -1, Proto Overdrive]
type = ChangeState
value = 3000
triggerall = command = "overdrive"
triggerall = power >= 1000
trigger1 = statetype != A
trigger1 = ctrl
trigger2 = stateno = 200 || stateno = 210 || stateno = 400 || stateno = 410
trigger2 = movecontact

[State -1, Proto Blast]
type = ChangeState
value = 1000
triggerall = command = "blast"
triggerall = NumProjID(1000) = 0
trigger1 = statetype != A
trigger1 = ctrl
trigger2 = stateno = 200 || stateno = 210 || stateno = 400 || stateno = 410
trigger2 = movecontact

[State -1, Run]
type = ChangeState
value = 100
trigger1 = command = "FF"
trigger1 = statetype = S
trigger1 = ctrl

[State -1, Hop back]
type = ChangeState
value = 105
trigger1 = command = "BB"
trigger1 = statetype = S
trigger1 = ctrl

[State -1, Taunt]
type = ChangeState
value = 195
trigger1 = command = "start"
trigger1 = statetype = S
trigger1 = ctrl

;-| Crouching attacks |---------------------------------------------------------
[State -1, Crouching strong kick]
type = ChangeState
value = 440
triggerall = command = "b"
triggerall = command = "holddown"
trigger1 = statetype != A
trigger1 = ctrl

[State -1, Crouching light kick]
type = ChangeState
value = 430
triggerall = command = "a"
triggerall = command = "holddown"
trigger1 = statetype != A
trigger1 = ctrl
trigger2 = stateno = 400
trigger2 = movecontact

[State -1, Crouching strong punch]
type = ChangeState
value = 410
triggerall = command = "y"
triggerall = command = "holddown"
trigger1 = statetype != A
trigger1 = ctrl
trigger2 = stateno = 400 || stateno = 430
trigger2 = movecontact

[State -1, Crouching light punch]
type = ChangeState
value = 400
triggerall = command = "x"
triggerall = command = "holddown"
trigger1 = statetype != A
trigger1 = ctrl
trigger2 = stateno = 400
trigger2 = time > 7

;-| Standing attacks |----------------------------------------------------------
[State -1, Strong kick]
type = ChangeState
value = 240
triggerall = command = "b"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl
trigger2 = stateno = 200 || stateno = 230
trigger2 = movecontact

[State -1, Light kick]
type = ChangeState
value = 230
triggerall = command = "a"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl
trigger2 = stateno = 200
trigger2 = movecontact

[State -1, Strong punch]
type = ChangeState
value = 210
triggerall = command = "y"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl
trigger2 = stateno = 200 || stateno = 230
trigger2 = movecontact

[State -1, Light punch]
type = ChangeState
value = 200
triggerall = command = "x"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl
trigger2 = stateno = 200
trigger2 = time > 7

;-| Air attacks |---------------------------------------------------------------
[State -1, Air strong kick]
type = ChangeState
value = 640
trigger1 = command = "b"
trigger1 = statetype = A
trigger1 = ctrl

[State -1, Air light kick]
type = ChangeState
value = 630
trigger1 = command = "a"
trigger1 = statetype = A
trigger1 = ctrl

[State -1, Air strong punch]
type = ChangeState
value = 610
trigger1 = command = "y"
trigger1 = statetype = A
trigger1 = ctrl

[State -1, Air light punch]
type = ChangeState
value = 600
trigger1 = command = "x"
trigger1 = statetype = A
trigger1 = ctrl
